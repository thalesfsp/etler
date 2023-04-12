# Use the official Golang image as the base image.
FROM golang:latest

# Set the current working directory inside the container.
WORKDIR /app

# Copy the source code into the container.
COPY . .

# Build the Go binary.
RUN go build -o etl-pipeline .

# Specify the command to run when the container starts.
CMD ["./etl-pipeline server"]