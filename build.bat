@echo off
echo Building...
go build -o bot.exe .
if %errorlevel% neq 0 (
    echo.
    echo BUILD FAILED!
    pause
    exit /b 1
)
echo.
echo BUILD SUCCESS! bot.exe siap dijalankan.
pause
