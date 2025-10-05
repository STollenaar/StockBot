package util

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/disgoorg/disgo/discord"
	"github.com/joho/godotenv"
	"github.com/stollenaar/aws-rotating-credentials-provider/credentials/filecreds"
)

type Config struct {
	DEBUG              bool
	DISCORD_TOKEN      string
	DUCKDB_PATH        string
	AWS_PARAMETER_NAME string

	AWS_REGION     string
	TERMINAL_REGEX string

	FINNHUB string
	AWS_FINNHUB string

	ADMIN_USER_ID string
}

var (
	ConfigFile *Config
	ssmClient  *ssm.Client
)

func init() {
	ConfigFile = new(Config)
	_, err := os.Stat(".env")
	if err == nil {
		err = godotenv.Load(".env")
		if err != nil {
			log.Fatal("Error loading environment variables")
		}
	}

	ConfigFile = &Config{
		AWS_REGION:         os.Getenv("AWS_REGION"),
		DISCORD_TOKEN:      os.Getenv("DISCORD_TOKEN"),
		AWS_PARAMETER_NAME: os.Getenv("AWS_PARAMETER_NAME"),
		TERMINAL_REGEX:     os.Getenv("TERMINAL_REGEX"),
		DUCKDB_PATH:        os.Getenv("DUCKDB_PATH"),
		ADMIN_USER_ID:      os.Getenv("ADMIN_USER_ID"),
		FINNHUB:            os.Getenv("FINNHUB"),
		AWS_FINNHUB:        os.Getenv("AWS_FINNHUB"),
	}
	if ConfigFile.TERMINAL_REGEX == "" {
		ConfigFile.TERMINAL_REGEX = `(\.|,|:|;|\?|!)$`
	}

}

func init() {

	if os.Getenv("AWS_SHARED_CREDENTIALS_FILE") != "" {
		provider := filecreds.NewFilecredentialsProvider(os.Getenv("AWS_SHARED_CREDENTIALS_FILE"))
		ssmClient = ssm.New(ssm.Options{
			Credentials: provider,
			Region:      ConfigFile.AWS_REGION,
		})
	} else {

		// Create a config with the credentials provider.
		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(ConfigFile.AWS_REGION),
		)

		if err != nil {
			if _, isProfileNotExistError := err.(config.SharedConfigProfileNotExistError); isProfileNotExistError {
				cfg, err = config.LoadDefaultConfig(context.TODO(),
					config.WithRegion(ConfigFile.AWS_REGION),
				)
			}
			if err != nil {
				log.Fatal("Error loading AWS config:", err)
			}
		}

		ssmClient = ssm.NewFromConfig(cfg)
	}
}

func GetDiscordToken() string {
	if ConfigFile.DISCORD_TOKEN == "" && ConfigFile.AWS_PARAMETER_NAME == "" {
		log.Fatal("DISCORD_TOKEN or AWS_PARAMETER_NAME is not set")
	}

	if ConfigFile.DISCORD_TOKEN != "" {
		return ConfigFile.DISCORD_TOKEN
	}
	out, err := getAWSParameter(ConfigFile.AWS_PARAMETER_NAME)
	if err != nil {
		log.Fatal(err)
	}
	return out
}

func GetFinnhub() (string, error) {
	if ConfigFile.FINNHUB == "" && ConfigFile.AWS_FINNHUB == "" {
		log.Fatal("OLLAMA_AUTH_USERNAME or AWS_OLLAMA_AUTH_USERNAME is not set")
	}

	if ConfigFile.FINNHUB != "" {
		return ConfigFile.FINNHUB, nil
	}
	return getAWSParameter(ConfigFile.AWS_FINNHUB)
}

func getAWSParameter(parameterName string) (string, error) {
	out, err := ssmClient.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           aws.String(parameterName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		fmt.Println(fmt.Errorf("error from fetching parameter %s. With error: %w", parameterName, err))
		return "", err
	}
	return *out.Parameter.Value, err
}

func (c *Config) SetEphemeral() discord.MessageFlags {
	if c.DEBUG {
		return discord.MessageFlagEphemeral
	} else {
		return discord.MessageFlagsNone
	}
}

func (c *Config) SetComponentV2Flags() *discord.MessageFlags {
	eph := c.SetEphemeral()
	eph = eph.Add(discord.MessageFlagIsComponentsV2)
	return &eph
}
