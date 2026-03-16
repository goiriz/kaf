package config

type Config struct {
	CurrentContext string             `yaml:"current-context"`
	Contexts       map[string]Context `yaml:"contexts"`
	LogPath        string             `yaml:"log_path,omitempty"`
	LogMaxSizeMB   int                `yaml:"log_max_size_mb,omitempty"`
	MaxMemoryMB    int                `yaml:"max_memory_mb,omitempty"`
}

type Context struct {
	Brokers           []string    `yaml:"brokers"`
	TLS               bool        `yaml:"tls,omitempty"`
	SASL              *SASLConfig `yaml:"sasl,omitempty"`
	WriteEnabled      bool        `yaml:"write,omitempty"`
	IsProduction      bool        `yaml:"is_production,omitempty"` // SAFETY: Enable strict confirmation for this context
	SchemaRegistryURL      string      `yaml:"schema_registry_url,omitempty"`
	SchemaRegistryUser     string      `yaml:"schema_registry_user,omitempty"`
	SchemaRegistryPass     string      `yaml:"schema_registry_pass,omitempty"`
	SchemaRegistryPassCmd  string      `yaml:"schema_registry_pass_cmd,omitempty"`
	SchemaRegistryInsecure bool        `yaml:"schema_registry_insecure,omitempty"`
	ProtoPaths             []string          `yaml:"proto_paths,omitempty"`
	TopicProtoMappings     map[string]string `yaml:"topic_proto_mappings,omitempty"`
	RedactKeys             []string          `yaml:"redact_keys,omitempty"`
}

type SASLConfig struct {
	Mechanism   string `yaml:"mechanism"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password,omitempty"`
	PasswordCmd string `yaml:"password_cmd,omitempty"`
}
