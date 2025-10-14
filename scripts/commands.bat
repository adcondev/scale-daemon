REM Comandos manuales en CMD con privilegios de administrador, usados en instalador

REM Stop the service
sc stop BasculaServicio

REM Start the service
sc start BasculaServicio

REM Restart (stop then start)
sc stop BasculaServicio && timeout /t 2 && sc start BasculaServicio

REM Check status
sc query BasculaServicio

REM Delete service
sc delete BasculaServicio

REM Install service
sc create BasculaServicio binPath= "C:\Path\To\Your\ServiceExecutable.exe