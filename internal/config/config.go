package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Silicon SiliconConfig `toml:"silicon"`
	Retry   RetryConfig   `toml:"retry"`
	Workers WorkersConfig `toml:"workers"`
	Hooks   HooksConfig   `toml:"hooks"`
}

type SiliconConfig struct {
	PollIntervalMS int `toml:"poll_interval_ms"`
}

type RetryConfig struct {
	CarbonBudget int `toml:"carbon_budget"`
	HeliumBudget int `toml:"helium_budget"`
	ReviewBudget int `toml:"review_budget"`
}

type WorkersConfig struct {
	CarbonConcurrency int      `toml:"carbon_concurrency"`
	HeliumConcurrency int      `toml:"helium_concurrency"`
	CarbonCommand     []string `toml:"carbon_command"`
	HeliumCommand     []string `toml:"helium_command"`
}

type HooksConfig struct {
	Enabled  bool   `toml:"enabled"`
	Lithium  string `toml:"lithium"`
	Chlorine string `toml:"chlorine"`
}

func Default() Config {
	return Config{
		Silicon: SiliconConfig{PollIntervalMS: 50},
		Retry:   RetryConfig{CarbonBudget: 3, HeliumBudget: 3, ReviewBudget: 2},
		Workers: WorkersConfig{CarbonConcurrency: 1, HeliumConcurrency: 1, CarbonCommand: []string{"echo", "carbon-stub"}, HeliumCommand: []string{"opencode", "run", "--agent", "helium"}},
		Hooks:   HooksConfig{Enabled: true, Lithium: filepath.ToSlash(filepath.Join(".molecular", "lithium.sh")), Chlorine: filepath.ToSlash(filepath.Join(".molecular", "chlorine.sh"))},
	}
}

var (
	ErrInvalid = errors.New("invalid config")
)

type LoadResult struct {
	Config     Config
	Found      bool
	Path       string
	ParseError error
}

func Load(repoRoot string) LoadResult {
	res := LoadResult{Config: Default()}
	path := filepath.Join(repoRoot, ".molecular", "config.toml")
	res.Path = path

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return res
		}
		res.ParseError = err
		return res
	}

	res.Found = true
	var parsed Config
	if err := toml.Unmarshal(b, &parsed); err != nil {
		res.ParseError = fmt.Errorf("%w: %v", ErrInvalid, err)
		return res
	}

	res.Config = merge(Default(), parsed)
	return res
}

func merge(def Config, cfg Config) Config {
	// Silicon
	if cfg.Silicon.PollIntervalMS != 0 {
		def.Silicon.PollIntervalMS = cfg.Silicon.PollIntervalMS
	}
	// Retry
	if cfg.Retry.CarbonBudget != 0 {
		def.Retry.CarbonBudget = cfg.Retry.CarbonBudget
	}
	if cfg.Retry.HeliumBudget != 0 {
		def.Retry.HeliumBudget = cfg.Retry.HeliumBudget
	}
	if cfg.Retry.ReviewBudget != 0 {
		def.Retry.ReviewBudget = cfg.Retry.ReviewBudget
	}
	// Workers
	if cfg.Workers.CarbonConcurrency != 0 {
		def.Workers.CarbonConcurrency = cfg.Workers.CarbonConcurrency
	}
	if cfg.Workers.HeliumConcurrency != 0 {
		def.Workers.HeliumConcurrency = cfg.Workers.HeliumConcurrency
	}
	if len(cfg.Workers.CarbonCommand) != 0 {
		def.Workers.CarbonCommand = cfg.Workers.CarbonCommand
	}
	if len(cfg.Workers.HeliumCommand) != 0 {
		def.Workers.HeliumCommand = cfg.Workers.HeliumCommand
	}
	// Hooks
	def.Hooks.Enabled = cfg.Hooks.Enabled
	if cfg.Hooks.Lithium != "" {
		def.Hooks.Lithium = cfg.Hooks.Lithium
	}
	if cfg.Hooks.Chlorine != "" {
		def.Hooks.Chlorine = cfg.Hooks.Chlorine
	}
	return def
}
