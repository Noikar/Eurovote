@echo off
rem Ensure mingw64\bin is in your PATH before running (required for CGO)
rem e.g. set PATH=C:\path\to\mingw64\bin;%PATH%
set CGO_ENABLED=1
echo Building eurovote.exe...
go build -ldflags="-H windowsgui -s -w" -o eurovote.exe .
if %ERRORLEVEL% == 0 (
    echo Done! eurovote.exe is ready.
) else (
    echo Build failed.
)
