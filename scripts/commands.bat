# Stop the service
sc stop BasculaServicio

# Start the service
sc start BasculaServicio

# Restart (stop then start)
sc stop BasculaServicio && timeout /t 2 && sc start BasculaServicio

# Check status
sc query BasculaServicio