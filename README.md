# kaf - High-Performance Kafka TUI

**kaf** is a robust, terminal-based Kafka client built for speed, safety, and modern data architectures. Unlike other tools, it's designed to be **Production-Safe by default**, preventing accidental data loss while providing deep insights into your clusters.

## Key Features

- 🛡️ **Safe by Default**: Starts in Read-Only mode. Destructive actions require an explicit `--write` flag.
- 📦 **Data-Aware**: Native support for **Avro** and **Protobuf** decoding via Confluent Schema Registry.
- 📂 **Local Protobuf Support**: Decode raw messages without a registry by pointing to your local `.proto` files.
- 🏷️ **Header Visibility**: View Kafka Headers at a glance for tracing and metadata debug.
- 🕵️ **Sensitive Data Masking**: Automatically redact keys like `password` or `token` from displayed JSON.
- 🧠 **Smart Memory Management**: Automatic buffer capping (100MB) and UI virtualization. It won't crash your server.
- 🕵️ **Real-time Insights**: View consumer group lag and status.
- 🚀 **Zero-Config**: Interactive setup wizard or environment variables support.

## Quick Start

### Installation

#### Pre-built Binaries (Linux, Windows, macOS)
Download the latest binary for your platform from the [Releases](https://github.com/goiriz/kaf/releases) page.

#### Go Install
```bash
go install github.com/goiriz/kaf/cmd/kaf@latest
```

#### From Source
```bash
make static
sudo cp kaf /usr/local/bin/
```

### Usage
Run the interactive setup wizard:
```bash
kaf setup
```

Launch with a specific broker:
```bash
kaf --broker localhost:9092
```
Or use environment variables:
```bash
export KAFKA_BROKERS="prod-1:9092,prod-2:9092"
kaf
```

## Protobuf Guide

`kaf` provides two ways to decode Protobuf messages without a Schema Registry:

### 1. Auto-Discovery (Guessing)
If you provide `proto_paths`, `kaf` will scan all `.proto` files and attempt to decode incoming messages by matching them against all known message types. This is great for exploration.

### 2. Strict Mode (Mapping)
To prevent incorrect guesses or to explicitly enforce a schema for a topic, use `topic_proto_mappings` in your config:
```yaml
topic_proto_mappings:
  "orders-topic": "com.example.Order"
  "users": "User"
```
In Strict Mode, if a message fails to decode as the mapped type, `kaf` will display a `(Decode Error)` and show the raw hex.

### 3. UI Overrides
While viewing a topic, press **`v`** to cycle through decoders:
- **AUTO (PROTO/AVRO)**: Uses Registry, Mappings, or Guessing.
- **TEXT/JSON**: Forces plain-text or pretty-printed JSON view.
- **HEX**: Shows the raw binary data.

`kaf` remembers your decoder choice per topic for the duration of the session.

## Keyboard Shortcuts

| Key | Action |
| :--- | :--- |
| `g` | Switch between Topics and Groups modes |
| `C` | **(Shift+C)** Switch Cluster Context |
| `tab` | Change focus between Sidebar and Details |
| `c` | **Tail** mode (Real-time stream) |
| `b` | **History** mode (Load last 1000 messages) |
| `y` | Copy message to clipboard |
| `v` | Cycle decoder (Auto/JSON -> Hex) |
| `?` | Show help menu |
| `esc` | Go back or Quit |

## Safety & Compliance

- **Anti-OOM Protection**: Message buffer is capped at 100MB by default.
- **Controller Protection**: Metadata requests have a 3s timeout to prevent DoS on large clusters.
- **Secure Secrets**: Supports `password_cmd` in configuration to fetch credentials from external vaults (e.g., `pass`, `aws-cli`).

## Configuration (~/.kaf.yaml)

```yaml
contexts:
  local:
    brokers: ["localhost:9092"]
  prod:
    brokers: ["kafka-prod:9092"]
    tls: true
    proto_paths: ["./protos", "/home/user/my-app/api"]
    sasl:
      mechanism: "SCRAM-SHA-512"
      username: "admin"
      password_cmd: "pass kafka/prod-password"
    schema_registry_url: "http://schema-registry:8081"
log_path: "/tmp/kaf.log"
log_max_size_mb: 10
```

---

