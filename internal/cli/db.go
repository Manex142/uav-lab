package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Manex142/uav-lab/internal/database"
	"github.com/spf13/cobra"
)

type dbOptions struct {
	host     string
	port     int
	user     string
	password string
	database string
	sslMode  string
}

func (o *dbOptions) toConfig() database.Config {
	cfg := database.DefaultConfig()
	if o.host != "" {
		cfg.Host = o.host
	}
	if o.port != 0 {
		cfg.Port = o.port
	}
	if o.user != "" {
		cfg.User = o.user
	}
	if o.password != "" {
		cfg.Password = o.password
	}
	if o.database != "" {
		cfg.Database = o.database
	}
	if o.sslMode != "" {
		cfg.SSLMode = o.sslMode
	}
	return cfg
}

func newDBCmd() *cobra.Command {
	opts := &dbOptions{}

	cmd := &cobra.Command{
		Use:   "db",
		Short: "Manage TimescaleDB persistence and migrations",
		Long:  `Subcommands for database operations, health checking, and migrations.`,
	}

	flags := cmd.PersistentFlags()
	flags.StringVar(&opts.host, "host", "127.0.0.1", "Host de TimescaleDB")
	flags.IntVar(&opts.port, "port", 5432, "Puerto de TimescaleDB")
	flags.StringVar(&opts.user, "user", "uav_admin", "Usuario de TimescaleDB")
	flags.StringVar(&opts.password, "password", "uav_password", "Contraseña de TimescaleDB")
	flags.StringVar(&opts.database, "database", "uav_telemetry", "Nombre de base de datos")
	flags.StringVar(&opts.sslMode, "sslmode", "disable", "Modo SSL")

	cmd.AddCommand(newDBPingCmd(opts))
	cmd.AddCommand(newDBMigrateCmd(opts))

	return cmd
}

func newDBPingCmd(opts *dbOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Check connection health to TimescaleDB",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := opts.toConfig()
			fmt.Printf("🐘 Comprobando conexión a TimescaleDB (%s:%d/%s)...\n", cfg.Host, cfg.Port, cfg.Database)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			pool, err := database.NewPool(ctx, cfg)
			if err != nil {
				return fmt.Errorf("fallo de conexión: %w", err)
			}
			defer pool.Close()

			fmt.Println("✅ Conexión establecida correctamente con TimescaleDB.")
			return nil
		},
	}
}

func newDBMigrateCmd(opts *dbOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Apply all pending SQL migrations with Goose",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := opts.toConfig()
			fmt.Printf("🐘 Ejecutando migraciones en TimescaleDB (%s:%d/%s)...\n", cfg.Host, cfg.Port, cfg.Database)

			if err := database.RunMigrations(cfg.DSN()); err != nil {
				return fmt.Errorf("fallo ejecutando migraciones: %w", err)
			}

			fmt.Println("✅ Todas las migraciones se han aplicado con éxito.")
			return nil
		},
	}
}
