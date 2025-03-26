package models

import (
	"github.com/google/uuid"
	"time"
)

type Job struct {
	ID                            int       `gorm:"primaryKey;autoIncrement"`
	OCROracleSpecID               *int      `gorm:"column:ocr_oracle_spec_id"`
	OCR2OracleSpecID              *int      `gorm:"column:ocr2_oracle_spec_id"`
	Name                          *string   `gorm:"type:varchar(255);index:idx_jobs_name"`
	SchemaVersion                 int       `gorm:"not null;check:schema_version > 0"`
	Type                          string    `gorm:"type:varchar(255);not null;check:type <> ''"`
	MaxTaskDuration               *int64    `gorm:"column:max_task_duration"`
	DirectRequestSpecID           *int      `gorm:"column:direct_request_spec_id"`
	FluxMonitorSpecID             *int      `gorm:"column:flux_monitor_spec_id"`
	KeeperSpecID                  *int      `gorm:"column:keeper_spec_id"`
	CronSpecID                    *int      `gorm:"column:cron_spec_id"`
	VRFSpecID                     *int      `gorm:"column:vrf_spec_id"`
	WebhookSpecID                 *int      `gorm:"column:webhook_spec_id"`
	BlockhashStoreSpecID          *int      `gorm:"column:blockhash_store_spec_id"`
	BlockHeaderFeederSpecID       *int      `gorm:"column:block_header_feeder_spec_id"`
	BootstrapSpecID               *int      `gorm:"column:bootstrap_spec_id"`
	GatewaySpecID                 *int      `gorm:"column:gateway_spec_id"`
	LegacyGasStationServerSpecID  *int      `gorm:"column:legacy_gas_station_server_spec_id"`
	LegacyGasStationSidecarSpecID *int      `gorm:"column:legacy_gas_station_sidecar_spec_id"`
	EALSpecID                     *int      `gorm:"column:eal_spec_id"`
	LiquidityBalancerSpecID       *int64    `gorm:"column:liquidity_balancer_spec_id"`
	WorkflowSpecID                *int      `gorm:"column:workflow_spec_id"`
	StandardCapabilitiesSpecID    *int      `gorm:"column:standard_capabilities_spec_id"`
	CCIPSpecID                    *int      `gorm:"column:ccip_spec_id"`
	CCIPBootstrapSpecID           *int      `gorm:"column:ccip_bootstrap_spec_id"`
	BALSpecID                     *int      `gorm:"column:bal_spec_id"`
	StreamID                      *int64    `gorm:"column:stream_id;index:idx_jobs_unique_stream_id,unique,where:stream_id IS NOT NULL"`
	ExternalJobID                 uuid.UUID `gorm:"type:uuid;not null;uniqueIndex;check:external_job_id <> '00000000-0000-0000-0000-000000000000'"`
	CreatedAt                     time.Time `gorm:"not null;index:idx_jobs_created_at,using:brin"`
	GasLimit                      *int64    `gorm:"column:gas_limit"`
	ForwardingAllowed             bool      `gorm:"not null;default:false"`

	// GORM soft delete or timestamps (if needed)
	// UpdatedAt time.Time
	// DeletedAt gorm.DeletedAt `gorm:"index"`

	// Foreign keys handled by GORM relationships if needed
}

// TableName overrides the default table name
func (Job) TableName() string {
	return "jobs"
}
