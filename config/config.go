package config

import (
	"log"

	"github.com/mgarnier-home/utils/common"
)

type AppConfigFile struct {
	KeepAliveUrl string            `yaml:"keepAliveUrl"`
	Mail         *MailConfig       `yaml:"mail"`
	FileName     string            `yaml:"fileName"`
	BackupSrc    string            `yaml:"backupSrc"`
	RemoteDest   *RemoteDestConfig `yaml:"remoteDest"`
	KeepDuration int               `yaml:"keepDuration"` // in days
}

type MailConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Login    string `yaml:"login"`
	Password string `yaml:"password"`
	InfoTo   string `yaml:"infoTo"`
	ErrorTo  string `yaml:"errorTo"`
}

type RemoteDestConfig struct {
	SSHHost string `yaml:"sshHost"`
	SSHPort int    `yaml:"sshPort"`
	SSHUser string `yaml:"sshUser"`
	SSHPath string `yaml:"sshPath"`
}

type AppEnvConfig struct {
	ServerPort     int
	ConfigFilePath string
	SSHPrivateKey  string

	AppConfig *AppConfigFile
}

func getAppEnvConfig() (appEnvConfig *AppEnvConfig) {
	common.InitEnvFromFile()

	appEnvConfig = &AppEnvConfig{
		ServerPort:     common.GetEnv("SERVER_PORT", 8080),
		ConfigFilePath: common.GetEnv("CONFIG_FILE_PATH", "./data/config.yaml"),
		SSHPrivateKey:  common.GetEnv("SSH_PRIVATE_KEY", ""),
	}

	var err error
	appEnvConfig.AppConfig, err = common.ReadYamlFile[AppConfigFile](appEnvConfig.ConfigFilePath)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
		panic(err)
	}

	appConfig := appEnvConfig.AppConfig
	if appConfig.Mail == nil || appConfig.FileName == "" || appConfig.BackupSrc == "" || appConfig.RemoteDest == nil {
		log.Fatalf("Invalid config file %s: mail, fileName, backupSrc and remoteDest are required", appEnvConfig.ConfigFilePath)
	}

	if appEnvConfig.AppConfig.KeepDuration == 0 {
		appEnvConfig.AppConfig.KeepDuration = 14 // default: 30 days
	}

	return appEnvConfig
}

var Config *AppEnvConfig

func Load() {
	Config = getAppEnvConfig()
}
