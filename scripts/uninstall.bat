@echo off
sc.exe stop "BasculaServicio"
sc.exe delete "BasculaServicio"
echo Servicio desinstalado con éxito!
pause