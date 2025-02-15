package config

// Config service.
type Config struct {
	ServiceName string `mapstructure:"service_name" default:"xFreeService"`
	PathDB      string `mapstructure:"path_db" default:"/db/xfree.db"`
	TimeToSend  string `mapstructure:"time_to_send" default:"18:00"`

	TBot TBot `mapstructure:"tbot"`
}

// TBot Telegram bot structure, used for integration telegram bot in service.
type TBot struct {
	AdminChatID  int64        `mapstructure:"admin_chat_id"`
	AccessToken  string       `mapstructure:"access_token" required:"true"`
	UpdateConfig UpdateConfig `mapstructure:"update_config"`
}

// UpdateConfig struct duplicate struct from package telegrap-bot-api.
type UpdateConfig struct {
	Offset  int `mapstructure:"offset"`
	Limit   int `mapstructure:"limit"`
	Timeout int `mapstructure:"timeout"`
}
