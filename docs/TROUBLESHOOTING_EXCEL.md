# Troubleshooting Excel Export Issues

## Common Issues and Solutions

### Issue 1: Excel File Not Created

**Symptoms:**
- No Excel file appears after running `-verify-excel`
- No error message shown

**Possible Causes:**

1. **Directory doesn't exist**
   ```powershell
   # Solution: Use absolute path or create directory first
   New-Item -ItemType Directory -Path "C:\Reports" -Force
   .\etl-worker.exe -verify-excel -excel-output "C:\Reports\verification.xlsx"
   ```

2. **No data to export**
   - Check if ETL completed successfully
   - Check if verification found any records
   - Look for "Total Records: 0" in output

3. **Permission issues**
   - Check if you have write permissions to the directory
   - Try saving to a different location (e.g., Desktop)

### Issue 2: "Syntax Error" When Running EXE

**Symptoms:**
- Error message appears when double-clicking exe
- "Syntax error" or similar message

**Causes:**

1. **Double-clicking the exe without arguments**
   - The exe needs command-line arguments
   - Solution: Run from PowerShell/Command Prompt

2. **Missing config file**
   ```powershell
   # Error: Failed to load config: ...
   # Solution: Create config.yaml or specify path
   .\etl-worker.exe -config config.yaml -verify-excel
   ```

3. **Invalid command syntax**
   ```powershell
   # Wrong: Missing quotes for paths with spaces
   .\etl-worker.exe -excel-output C:\My Reports\file.xlsx
   
   # Correct: Use quotes
   .\etl-worker.exe -excel-output "C:\My Reports\file.xlsx"
   ```

### Issue 3: Excel File Created But Empty

**Symptoms:**
- Excel file exists but has no data
- Only headers visible

**Causes:**

1. **No records found during verification**
   - Check source database has data
   - Check destination database has encrypted data
   - Verify database connections

2. **Verification failed silently**
   - Check console output for errors
   - Look for "Verified 0 records" message

## Step-by-Step Debugging

### Step 1: Verify EXE Works

```powershell
# Test 1: Check help
.\etl-worker.exe -help

# Should show:
# -config string
# -excel-output string
# -verify
# -verify-excel
```

### Step 2: Check Config File

```powershell
# Test 2: Verify config file exists and is valid
if (Test-Path ".\config.yaml") {
    Get-Content .\config.yaml
} else {
    Write-Host "ERROR: config.yaml not found!"
}
```

### Step 3: Test Basic ETL First

```powershell
# Test 3: Run normal ETL first (without verification)
.\etl-worker.exe -config config.yaml

# Check if data is encrypted and stored
```

### Step 4: Test Verification Mode

```powershell
# Test 4: Run verification with explicit path
.\etl-worker.exe `
    -verify-excel `
    -excel-output ".\verification_test.xlsx" `
    -config config.yaml

# Check console output for:
# - "Step 1: Running ETL (Encryption)"
# - "Step 2: Verifying All Encrypted Data"
# - "Step 3: Exporting Results to Excel"
# - "Excel file saved successfully"
```

### Step 5: Check Excel File Location

```powershell
# Test 5: Verify file was created
$excelPath = ".\verification_test.xlsx"
if (Test-Path $excelPath) {
    Write-Host "✓ Excel file created: $excelPath" -ForegroundColor Green
    $file = Get-Item $excelPath
    Write-Host "  Size: $($file.Length) bytes" -ForegroundColor Gray
    Write-Host "  Created: $($file.CreationTime)" -ForegroundColor Gray
} else {
    Write-Host "✗ Excel file NOT found: $excelPath" -ForegroundColor Red
}
```

## Correct Usage Examples

### Example 1: Basic Usage (Current Directory)

```powershell
.\etl-worker.exe -verify-excel -config config.yaml
# Creates: verification_results.xlsx in current directory
```

### Example 2: Custom Path

```powershell
.\etl-worker.exe `
    -verify-excel `
    -excel-output "C:\Reports\verification_20250112.xlsx" `
    -config config.yaml
```

### Example 3: With Timestamp

```powershell
$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
.\etl-worker.exe `
    -verify-excel `
    -excel-output ".\verification_$timestamp.xlsx" `
    -config config.yaml
```

### Example 4: Full Command with Token

```powershell
$env:KMS_BEARER_TOKEN = "your-token-here"
.\etl-worker.exe `
    -verify-excel `
    -excel-output ".\results.xlsx" `
    -config config.yaml
```

## Error Messages Reference

| Error Message | Cause | Solution |
|---------------|-------|----------|
| `Failed to load config` | config.yaml missing/invalid | Create valid config.yaml |
| `Failed to export to Excel` | Directory doesn't exist or permission denied | Create directory or use different path |
| `No verification results to export` | No data found | Check databases have data |
| `Failed to query encrypted data` | Database connection issue | Check database connection strings |
| `Failed to save Excel file` | File locked or permission issue | Close Excel if open, check permissions |

## Quick Fix Checklist

- [ ] EXE file exists and is not corrupted
- [ ] config.yaml exists and is valid YAML
- [ ] Database connections work (test separately)
- [ ] KMS server is running
- [ ] JWT token is set (if using auth)
- [ ] Output directory exists (or use current directory)
- [ ] Have write permissions to output location
- [ ] Excel file is not open in another program

## Still Having Issues?

1. **Check console output** - Look for error messages
2. **Run with verbose logging** - Check all log messages
3. **Test step by step**:
   - First: Normal ETL (without verification)
   - Second: Verification mode (without Excel)
   - Third: Full verification with Excel
4. **Check file permissions** - Try saving to Desktop
5. **Verify data exists** - Check databases have records

