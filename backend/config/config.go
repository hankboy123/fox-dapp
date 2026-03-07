package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Host string `mapstructure:"host"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
	Expire string `mapstructure:"expire"`
}

func LoadSimple() *Config {
	// 简化配置加载，实际应该使用Viper
	return &Config{
		Server: ServerConfig{
			Port: "8080",
			Host: "0.0.0.0",
			Mode: "debug",
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			Username: "root",
			Password: "password",
			DBName:   "mydb",
		},
		JWT: JWTConfig{
			Secret: "your-secret-key-change-in-production",
			Expire: "24h",
		},
	}
}

var GlobalConfig Config

func Load(configPath string) *Config {
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("config.yaml")
		viper.AddConfigPath("./")
		viper.AddConfigPath("../")
		viper.AddConfigPath("../../")

	}

	viper.SetConfigType("yaml")
	viper.AutomaticEnv()
	viper.SetEnvPrefix("SH_MANAGE")

	//环境标量使用下划线替换点
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %w", err))
	}

	if error := viper.Unmarshal(&GlobalConfig); error != nil {
		panic(fmt.Errorf("解析配置文件失败: %w", error))
	}

	// 监听配置文件变化
	viper.WatchConfig()
	viper.OnConfigChange(func(in fsnotify.Event) {
		fmt.Println("配置文件已修改:", in.Name)
		if error := viper.Unmarshal(&GlobalConfig); error != nil {
			fmt.Println("解析配置文件失败:", error)
		}
	})

	log.Println("配置文件加载成功:", viper.ConfigFileUsed())
	return &GlobalConfig

}

func GetMySQLDSN(config *Config) string {
	user := config.Database.Username
	pass := config.Database.Password
	host := config.Database.Host
	port := config.Database.Port
	dbname := config.Database.DBName

	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, pass, host, port, dbname)
}
