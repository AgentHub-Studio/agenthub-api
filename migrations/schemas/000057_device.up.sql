-- Device Node Network: registry of MCP-discoverable devices and sensors.
-- Stored in ah_{tenantID}.device — no tenant_id column.
--
-- Devices are discovered by the MCP Client Runtime and registered here.
-- An agent can subscribe to a device to receive its data via MCP Resources.

CREATE TABLE device (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    -- type: SENSOR, ACTUATOR, GATEWAY, COMPUTE
    type            VARCHAR(50) NOT NULL DEFAULT 'SENSOR',
    description     TEXT        NOT NULL DEFAULT '',
    -- mcp_server_config_id links to the MCP server that exposes this device.
    mcp_server_config_id UUID   REFERENCES mcp_server_config(id) ON DELETE SET NULL,
    -- resource_uri is the MCP Resource URI used to read device data.
    resource_uri    VARCHAR(1024) NOT NULL DEFAULT '',
    -- capabilities is a JSON array of strings describing what the device can do.
    capabilities    JSONB       NOT NULL DEFAULT '[]',
    -- last_seen_at is updated when the device last reported data.
    last_seen_at    TIMESTAMPTZ,
    -- status: ONLINE, OFFLINE, UNKNOWN
    status          VARCHAR(20) NOT NULL DEFAULT 'UNKNOWN',
    -- metadata holds device-specific key/value pairs (firmware version, location, etc.)
    metadata        JSONB       NOT NULL DEFAULT '{}',
    enabled         BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- agent_device links agents to devices they can interact with.
CREATE TABLE agent_device (
    agent_id    UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    device_id   UUID NOT NULL REFERENCES device(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, device_id)
);

CREATE INDEX idx_device_mcp_server  ON device(mcp_server_config_id);
CREATE INDEX idx_device_status      ON device(status);
CREATE INDEX idx_agent_device_agent ON agent_device(agent_id);
