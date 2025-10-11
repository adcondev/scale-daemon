@echo off
REM set GOPATH if needed, e.g., set GOPATH=%USERPROFILE%\go
go build -o ./bin/BasculaServicio.exe ./cmd/BasculaServicio
sc.exe create "BasculaServicio" binPath= "%CD%\BasculaServicio.exe" start= auto
sc.exe description "BasculaServicio" "Servicio Websocket y Serial para báscula"
sc.exe start "BasculaServicio"
echo Servicio instalado e iniciado con éxito!
pause