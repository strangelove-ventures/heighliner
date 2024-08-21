FROM cosmos-heighliner:cosmoshub-v18.1.0

# Copy the new gaiad binary into the container
COPY ./gaiad-v19.1.0-linux-amd64 /bin/gaiad

# Ensure the binary is executable
RUN chmod +x /bin/gaiad
