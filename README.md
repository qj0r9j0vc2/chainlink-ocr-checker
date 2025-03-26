# chainlink-ocr-checker

## Quickstart

```bash 
cat <<EOF > config.toml
log_level = "info"
chain_id = 137
rpc_addr = "https://polygon.drpc.org"


[database] # Only needed to run watch sub-command
user = '' 
password = '' 
host = 'localhost'
port = '5434'
dbName = 'ocrdb'

sslMode = 'disable'
EOF



go build -o ocr-checker .
./ocr-checker watch 0x2dbbd12bf0f6a23cf4455cc6be874b7a246288ce 10
# ./ocr-checker watch [transmitter] [count to check last rounds] [days to ignore if no rounds are there within from today]

./ocr-checker fetch 0xa142BB41f409599603D3bB16842D0d274AAeDcf5 1 2148
# ./ocr-checker watch [contract] [start_round] [until_round]

./ocr-checker parse results/0xa142BB41f409599603D3bB16842D0d274AAeDcf5-1_2148.yaml day > results/0xa142BB41f409599603D3bB16842D0d274AAeDcf5-1_2148.txt
# ./ocr-checker parse [fetched_data] [unit_to_show; day, month] > [output_file_path]
```
