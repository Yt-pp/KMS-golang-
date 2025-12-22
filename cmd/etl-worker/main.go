package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	kmsproto "kms/proto"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/microsoft/go-mssqldb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"gopkg.in/yaml.v3"
)

// This is a simplified ETL worker that:
// 1. Reads card data from a source DB table.
// 2. Calls the KMS gRPC service to encrypt PAN and CVV.
// 3. Writes the encrypted values into a target DB table.
//
// You should adapt the SQL queries and connection strings for your real schema
// and for the three different source databases.

type CardRecord struct {
	ID        int64
	CardNo    string
	CVV       string
	OtherData string
}

type DBConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type AppConfig struct {
	KMS struct {
		Addr string `yaml:"addr"`
	} `yaml:"kms"`
	Auth struct {
		BearerToken string `yaml:"bearerToken"`
	} `yaml:"auth"`
	SourceDB DBConfig `yaml:"sourceDB"`
	DestDB   DBConfig `yaml:"destDB"`
}

func main() {
	cfgPath := getenvDefault("ETL_CONFIG_PATH", "config.yaml")

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("failed to load config from %s: %v", cfgPath, err)
	}

	srcDriver := cfg.SourceDB.Driver
	srcDSN := cfg.SourceDB.DSN
	dstDriver := cfg.DestDB.Driver
	dstDSN := cfg.DestDB.DSN
	kmsAddr := cfg.KMS.Addr

	// Prefer token from env (fresh from Login); fallback to config if set there.
	kmsToken := os.Getenv("KMS_BEARER_TOKEN")
	if kmsToken == "" {
		kmsToken = cfg.Auth.BearerToken
	}

	srcDB, err := sql.Open(srcDriver, srcDSN)
	if err != nil {
		log.Fatalf("failed to open source DB: %v (driver=%s)", err, srcDriver)
	}
	defer srcDB.Close()

	dstDB, err := sql.Open(dstDriver, dstDSN)
	if err != nil {
		log.Fatalf("failed to open destination DB: %v (driver=%s)", err, dstDriver)
	}
	defer dstDB.Close()

	conn, err := grpc.Dial(kmsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to dial KMS: %v", err)
	}
	defer conn.Close()

	kmsClient := kmsproto.NewKMSClient(conn)

	records, err := loadCardRecords(srcDB)
	if err != nil {
		log.Fatalf("failed to load card records: %v", err)
	}

	for _, r := range records {
		if err := processRecord(dstDB, kmsClient, &r, kmsToken, cfg.DestDB.Driver); err != nil {
			log.Printf("failed to process record id=%d: %v", r.ID, err)
		}
	}
}

func loadCardRecords(db *sql.DB) ([]CardRecord, error) {
	rows, err := db.Query(`
		SELECT id, card_no, cvv, other_data
		FROM cards_to_encrypt
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []CardRecord
	for rows.Next() {
		var r CardRecord
		if err := rows.Scan(&r.ID, &r.CardNo, &r.CVV, &r.OtherData); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}

func processRecord(dstDB *sql.DB, kmsClient kmsproto.KMSClient, r *CardRecord, token string, driver string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}

	encPAN, err := kmsClient.Encrypt(ctx, &kmsproto.EncryptRequest{
		Plaintext: []byte(r.CardNo),
	})
	if err != nil {
		return err
	}

	encCVV, err := kmsClient.Encrypt(ctx, &kmsproto.EncryptRequest{
		Plaintext: []byte(r.CVV),
	})
	if err != nil {
		return err
	}

	// Store ciphertext and nonce in destination DB.
	insertSQL := insertStatementForDriver(driver)

	_, err = dstDB.Exec(insertSQL,
		sql.Named("source_id", r.ID),
		sql.Named("pan_ciphertext", encPAN.Ciphertext),
		sql.Named("pan_nonce", encPAN.Nonce),
		sql.Named("cvv_ciphertext", encCVV.Ciphertext),
		sql.Named("cvv_nonce", encCVV.Nonce),
		sql.Named("other_data", r.OtherData),
	)
	return err
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loadConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// insertStatementForDriver returns an INSERT statement with parameter markers
// suitable for the given driver name.
func insertStatementForDriver(driver string) string {
	switch driver {
	case "sqlserver":
		// SQL Server uses named params like @p1 or @name. Using names for clarity.
		return `
			INSERT INTO encrypted_cards
				(source_id, pan_ciphertext, pan_nonce, cvv_ciphertext, cvv_nonce, other_data)
			VALUES (@source_id, @pan_ciphertext, @pan_nonce, @cvv_ciphertext, @cvv_nonce, @other_data)
		`
	case "mysql":
		// MySQL does not support named parameters, but the driver will map Named args to '?'
		// in positional order.
		return `
			INSERT INTO encrypted_cards
				(source_id, pan_ciphertext, pan_nonce, cvv_ciphertext, cvv_nonce, other_data)
			VALUES (?, ?, ?, ?, ?, ?)
		`
	default:
		// Default to positional parameters.
		return fmt.Sprintf(`
			INSERT INTO encrypted_cards
				(source_id, pan_ciphertext, pan_nonce, cvv_ciphertext, cvv_nonce, other_data)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
	}
}


