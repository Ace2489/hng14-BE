package utils

import (
	"fmt"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

type Config struct {
	Addr  string
	Port  int
	Redis *redis.Client
	Gh    *GithubOauth
}

type GithubOauth struct {
	ClientId     string
	ClientSecret string
	RedirectUri  string
}

func (c *Config) Load() error {
	addr, ok := os.LookupEnv("API_HOST")
	if !ok || addr == "" {
		addr = "127.0.0.1"
	}
	c.Addr = addr

	portStr, ok := os.LookupEnv("API_PORT")
	port, _ := strconv.Atoi(portStr)

	if port == 0 {
		port = 3000
	}
	c.Port = port

	//Redis initialisation
	redisUrl, ok := os.LookupEnv("REDIS_URL")
	if !ok {
		return fmt.Errorf("REDIS_URL is not set")
	}

	opt, err := redis.ParseURL(redisUrl)
	if err != nil {
		return fmt.Errorf("invalid REDIS_URL: %w", err)
	}

	// BUG: This is an issue from redis/go-redis. Update once fixed
	// https://github.com/redis/go-redis/issues/3536
	opt.MaintNotificationsConfig = &maintnotifications.Config{
		Mode: maintnotifications.ModeDisabled,
	}

	c.Redis = redis.NewClient(opt)

	ghClient, ok := os.LookupEnv("GITHUB_CLIENT_ID")
	if !ok || ghClient == "" {
		return fmt.Errorf("GITHUB_CLIENT_ID not set")
	}
	ghSecret, ok := os.LookupEnv("GITHUB_CLIENT_SECRET")
	if !ok || ghSecret == "" {
		return fmt.Errorf("GITHUB_CLIENT_SECRET not set")
	}

	c.Gh = &GithubOauth{ClientId: ghClient, ClientSecret: ghSecret, RedirectUri: "http://localhost:3000/api/v1/auth/github/callback"}

	return nil
}
