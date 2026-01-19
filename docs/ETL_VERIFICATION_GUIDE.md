# ETL Worker Verification & Excel Export Guide

## Overview

The ETL worker now supports automatic verification after encryption. It will:
1. Encrypt all data from source database
2. Decrypt all encrypted data from destination database
3. Compare decrypted data with original data
4. Export detailed comparison results to Excel

## Usage

### Basic Usage

```bash
# Run ETL with verification and Excel export
.\etl-worker.exe -verify-excel -excel-output verification_results.xlsx -config config.yaml
```

### Command Line Flags

- `-verify-excel`: Enable ETL + verification + Excel export mode
- `-excel-output`: Path to Excel output file (default: `verification_results.xlsx`)
- `-config`: Path to config file (default: `config.yaml`)

### Example

```powershell
# Set environment variables
$env:KMS_BEARER_TOKEN = "your-token-here"

# Run ETL with verification
.\etl-worker.exe -verify-excel -excel-output "C:\Reports\verification_$(Get-Date -Format 'yyyyMMdd_HHmmss').xlsx" -config config.yaml
```

## Excel Output Format

The Excel file contains two sheets:

### 1. Verification Results Sheet

Contains detailed comparison for each record:

| Column | Description |
|--------|-------------|
| Source ID | Original record ID |
| Original PAN | Original card number (masked) |
| Decrypted PAN | Decrypted card number (masked) |
| PAN Match | Yes/No |
| PAN Error | Error message if decryption failed |
| Original CVV | Original CVV (masked) |
| Decrypted CVV | Decrypted CVV (masked) |
| CVV Match | Yes/No |
| CVV Error | Error message if decryption failed |
| Other Data | Other metadata |

**Color Coding:**
- 🟢 **Green**: Perfect match (both PAN and CVV match)
- 🟡 **Yellow**: Mismatch (data doesn't match)
- 🔴 **Red**: Error (decryption failed)

### 2. Summary Sheet

Contains overall statistics:

- Total Records
- Perfect Matches
- PAN Matches
- CVV Matches
- Errors/Mismatches
- Success Rate (%)

## Workflow

```
┌─────────────────┐
│  Source DB      │
│  (cards_to_     │
│   encrypt)      │
└────────┬────────┘
         │
         │ Read all records
         ▼
┌─────────────────┐
│  Step 1: ETL    │
│  Encrypt all    │
│  data           │
└────────┬────────┘
         │
         │ Write encrypted data
         ▼
┌─────────────────┐
│  Destination DB │
│  (encrypted_    │
│   cards)        │
└────────┬────────┘
         │
         │ Read encrypted records
         ▼
┌─────────────────┐
│  Step 2: Verify │
│  Decrypt all    │
│  Compare with   │
│  original       │
└────────┬────────┘
         │
         │ Generate results
         ▼
┌─────────────────┐
│  Step 3: Export │
│  Excel file     │
│  with results   │
└─────────────────┘
```

## Performance

- Verification runs in parallel (20 workers by default)
- Processes records in batches
- Progress is displayed during verification
- Typical performance: 1000-2000 records/minute (depends on KMS server)

## Security Notes

- **PAN and CVV are masked** in Excel output (only last 4 digits shown)
- Original sensitive data is never written to Excel
- Only comparison results and masked values are exported
- Excel file should be stored securely

## Troubleshooting

### No Excel File Generated

- Check file path permissions
- Ensure directory exists
- Check disk space

### Verification Errors

- Check KMS server is running
- Verify JWT token is valid
- Check database connections
- Review error messages in Excel file

### Mismatches Found

- Verify KMS master key hasn't changed
- Check for data corruption
- Review encryption/decryption logs
- Ensure same KMS instance used for encrypt/decrypt

## Example Output

```
=== Step 1: Running ETL (Encryption) ===
Starting Production Batch ETL (Batch Size: 500)...
=== ETL Completed Successfully ===
Total Time:    2m15s
Total Records: 5000
Errors:        0

=== Step 2: Verifying All Encrypted Data ===
Verified 5000 records total.

=== Step 3: Exporting Results to Excel ===

=== Complete Verification Finished ===
Total Time:        5m30s
Total Records:     5000
PAN Matches:       5000
CVV Matches:       5000
Perfect Matches:   5000
Excel File:        verification_results.xlsx
```

## Related Commands

- `-verify`: Quick verification mode (console output only, no Excel)
- Normal mode: Just encrypt, no verification

