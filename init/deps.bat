@echo off
REM ============================================
REM  Instalador de Dependencias - Sistema Báscula
REM ============================================

echo [+] Instalando Go 1.24.6...
winget install --id=GoLang.Go -v "1.24.6" -e --silent

echo [+] Instalando Task...
winget install Task.Task --silent

echo [+] Configurando variables de entorno...
REM Detectar directorio de usuario actual
for /f "tokens=*" %%i in ('echo %USERPROFILE%') do set USER_DIR=%%i

REM Configurar GOPATH y GOROOT
setx GOPATH "%USER_DIR%\go" >nul 2>&1
setx GOROOT "%ProgramFiles%\Go" >nul 2>&1

REM Actualizar PATH
set "NEW_PATH=%PATH%;%USER_DIR%\go\bin;%ProgramFiles%\Go\bin"
setx PATH "%NEW_PATH%" >nul 2>&1

echo [+] Instalando dependencias de Go...
cd /d "%~dp0"
go mod download
go mod tidy

echo.
echo [OK] Instalación completada!
echo.
echo Próximos pasos:
echo   1. Reiniciar terminal/IDE
echo   2. Ejecutar: task --list
echo   3. Compilar: task build:all
echo.
pause