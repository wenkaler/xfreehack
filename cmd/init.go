package main

import (
	"log"

	"github.com/spf13/viper"
)

func init() {
	// Set default values
	viper.SetDefault("service_name", "xFreeService")
	viper.SetDefault("path_db", "xfree_database_file.db")
	viper.SetDefault("time_to_send", "18:00")
	viper.SetDefault("telegram.update_time", "60")

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// Read configuration from file
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("viper.ReadInConfig", err)
	}

	// Bind environment variables (if they exist)
	viper.AutomaticEnv()
}
