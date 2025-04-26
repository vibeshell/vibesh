#!/bin/bash

# Check if we're running in a devcontainer
if [ -f /.dockerenv ]; then
    echo "Running in DevContainer..."
    # DevContainer environment variables should already be set from .env.devcontainer
else
    # Check if a .env file exists and source it for environment variables
    if [ -f .env ]; then
        echo "Loading environment variables from .env file..."
        export $(grep -v '^#' .env | xargs)
    fi
fi

# Check if OpenAI API key is set
if [ -z "$OPENAI_API_KEY" ]; then
    echo "Warning: OPENAI_API_KEY is not set. AI and RAG modes will have limited functionality."
fi

# Build and run the Vibesh
echo "Building Vibesh..."
go build -o vibesh

if [ $? -eq 0 ]; then
    echo "Starting Vibesh..."
    ./vibesh
else
    echo "Build failed. Please check for errors."
    exit 1
fi
