# PowerShell script to regenerate protobuf files
# Usage: .\regenerate-proto.ps1

Write-Host "Regenerating protobuf files..."

cd proto
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative kms.proto

if ($LASTEXITCODE -eq 0) {
    Write-Host "Proto files regenerated successfully!"
} else {
    Write-Host "Error: Failed to regenerate proto files"
    exit 1
}

cd ..
Write-Host "Done!"

