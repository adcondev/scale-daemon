REM Instalar Golang 1.24.6
winget install --id=GoLang.Go -v "1.24.6" -e
REM Dependencias de desarrollo
winget install Task.Task
REM Una vez instalado Task: task --list o ver el Taskfile.yml, dependencias de Golang:
task deps
REM Variables de Entorno
setx USERPROFILE "C:\Users\RED 2000" /M
setx GOPATH "%USERPROFILE%\go" /M
setx GOROOT "C:\Program Files\Go" /M
setx PATH "%PATH%;%GOPATH%\bin;C:\Program Files\Go\bin" /M
REM Path a la carpeta bin del proyecto
setx CD "%USERPROFILE%\GolandProjects\daemonize-example\bin" /M