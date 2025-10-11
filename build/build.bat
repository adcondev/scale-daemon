@echo off
echo ========================================
echo  Compilando Instalador de BasculaServicio
echo ========================================
echo.

REM Step 1: Build the service executable
echo [1/3] Compilando servicio...
cd ..\cmd\service
go build -ldflags="-w -s -H=windowsgui" -o ..\..\temp\BasculaServicio.exe main.go
if errorlevel 1 (
    echo ERROR: Fallo al compilar el servicio
    pause
    exit /b 1
)

REM Step 2: Copy service to installer directory for embedding
echo [2/3] Preparando archivos para embebir...
copy ..\..\temp\BasculaServicio.exe ..\installer\BasculaServicio.exe >nul

REM Step 3: Build the installer with embedded service
echo [3/3] Compilando instalador con servicio embebido...
cd ..\installer
go build -ldflags="-w -s" -o ..\..\BSInstaller.exe main.go
if errorlevel 1 (
    echo ERROR: Fallo al compilar el instalador
    pause
    exit /b 1
)

REM Cleanup
cd ..\..\
del temp\BasculaServicio.exe >nul 2>&1
del cmd\installer\BasculaServicio.exe >nul 2>&1
rmdir temp >nul 2>&1

echo.
echo ========================================
echo  ✓ Compilacion exitosa!
echo ========================================
echo.
echo Archivo generado: BSInstaller.exe
echo Tamaño:
dir /b BSInstaller.exe | find "BSInstaller"
echo.

REM Optional: Compress with UPX if available
where upx >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo Comprimiendo ejecutable con UPX...
    upx --best --lzma BSInstaller.exe >nul 2>&1
    echo Tamaño después de comprimir:
    dir /b BSInstaller.exe | find "BSInstaller"
)

echo.
echo Instalador listo para distribuir!
pause