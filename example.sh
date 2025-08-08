#!/bin/sh

export ENV_PROMPT='Get current storage/mem info and write brief health report'
export ENV_PROMPT='Get storage/mem info and answer in json format'

export ENV_MODEL_NAME=llama3.2:3b
export ENV_MODEL_NAME=mistral:7b

./go-sysinfo2ollama | jq .
