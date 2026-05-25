@echo off
echo ========================================
echo   BongBot - REBUILD
echo ========================================
echo.

REM Stop bot.exe kalau lagi jalan
echo [1/4] Stopping bot.exe (kalau ada yang jalan)...
taskkill /F /IM bot.exe >nul 2>&1
timeout /t 1 /nobreak >nul

REM Hapus exe lama
echo [2/4] Menghapus bot.exe lama...
if exist bot.exe del /F /Q bot.exe

REM Bersihkan cache build (opsional, biar fresh)
echo [3/4] Cleaning build cache...
go clean

REM Build ulang
echo [4/4] Building bot.exe...
go build -o bot.exe .
if %errorlevel% neq 0 (
    echo.
    echo ========================================
    echo   REBUILD FAILED!
    echo ========================================
    pause
    exit /b 1
)

echo.
echo ========================================
echo   REBUILD SUCCESS!
echo ========================================
echo bot.exe siap dijalankan. Double-click start.bat
pause
