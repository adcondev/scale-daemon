@echo off
set GOPATH=C:\Users\RED 2000\go
go build -o BasculaServicio.exe
sc.exe create "BasculaServicio" binPath= "C:\Users\RED 2000\GolandProjects\daemonize-example\BasculaServicio.exe" start= auto
sc.exe description "BasculaServicio" "Servicio Websocket y Serial para báscula"
sc.exe start "BasculaServicio"
echo Servicio instalado e iniciado con éxito!
pause