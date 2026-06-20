package vpnresource_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Ensure strings import is used.
var _ = strings.NewReader

type mockVpnRepo struct {
	data map[uuid.UUID]vpnresource.VpnResource
}

func newMockRepo() *mockVpnRepo {
	return &mockVpnRepo{data: make(map[uuid.UUID]vpnresource.VpnResource)}
}

func (m *mockVpnRepo) ListAll(_ context.Context, _ string, _ pagination.PageRequest) ([]vpnresource.VpnResource, int, error) {
	out := make([]vpnresource.VpnResource, 0, len(m.data))
	for _, v := range m.data {
		out = append(out, v)
	}
	return out, len(out), nil
}

func (m *mockVpnRepo) GetByID(_ context.Context, _ string, id uuid.UUID) (vpnresource.VpnResource, error) {
	v, ok := m.data[id]
	if !ok {
		return vpnresource.VpnResource{}, vpnresource.ErrNotFound
	}
	return v, nil
}

func (m *mockVpnRepo) Create(_ context.Context, _ string, v vpnresource.VpnResource) (vpnresource.VpnResource, error) {
	v.ID = uuid.New()
	m.data[v.ID] = v
	return v, nil
}

func (m *mockVpnRepo) Update(_ context.Context, _ string, id uuid.UUID, v vpnresource.VpnResource) (vpnresource.VpnResource, error) {
	if _, ok := m.data[id]; !ok {
		return vpnresource.VpnResource{}, vpnresource.ErrNotFound
	}
	v.ID = id
	m.data[id] = v
	return v, nil
}

func (m *mockVpnRepo) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return vpnresource.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockVpnRepo) ExistsByName(_ context.Context, _ string, name string) (bool, error) {
	for _, v := range m.data {
		if v.Name == name {
			return true, nil
		}
	}
	return false, nil
}

const tenantID = "test-tenant"

const validOvpn = `client
dev tun
proto udp
remote vpn.example.com 1194
resolv-retry infinite
nobind
persist-key
persist-tun
ca ca.crt
cert client.crt
key client.key
`

func TestVpnService_Create_Success(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	v, err := svc.Create(context.Background(), tenantID, vpnresource.CreateRequest{
		Name:           "Corp VPN",
		OvpnConfigPath: "/etc/vpn/corp.ovpn",
	})
	require.NoError(t, err)
	assert.Equal(t, "Corp VPN", v.Name)
	assert.NotEqual(t, uuid.Nil, v.ID)
}

func TestVpnService_GetByID_NotFound(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, vpnresource.ErrNotFound)
}

func TestVpnService_Delete_NotFound(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	err := svc.Delete(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, vpnresource.ErrNotFound)
}

func TestVpnService_Update_Success(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, vpnresource.CreateRequest{Name: "Old", OvpnConfigPath: "/tmp/x.ovpn"})
	require.NoError(t, err)
	updated, err := svc.Update(context.Background(), tenantID, created.ID, vpnresource.CreateRequest{Name: "New", OvpnConfigPath: "/tmp/x.ovpn"})
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
}

func TestVpnService_ListAll(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	for i := 0; i < 2; i++ {
		_, err := svc.Create(context.Background(), tenantID, vpnresource.CreateRequest{Name: "VPN", OvpnConfigPath: "/tmp/x.ovpn"})
		require.NoError(t, err)
	}
	items, total, err := svc.ListAll(context.Background(), tenantID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, items, 2)
}

// ---- TestConnection ----

func TestVpnService_TestConnection_NoConfig_ReturnsFalse(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, vpnresource.CreateRequest{Name: "VPN"})
	require.NoError(t, err)

	// VPN without an uploaded .ovpn config cannot connect.
	res, err := svc.TestConnection(context.Background(), tenantID, created.ID)
	require.NoError(t, err)
	assert.False(t, res.Connected)
	assert.NotEmpty(t, res.Message)
}

func TestVpnService_TestConnection_NotFound(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	_, err := svc.TestConnection(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, vpnresource.ErrNotFound)
}

// ---- UploadOvpnConfig ----

func TestVpnService_UploadOvpnConfig_ValidFile(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, vpnresource.CreateRequest{Name: "VPN"})
	require.NoError(t, err)

	updated, err := svc.UploadOvpnConfig(context.Background(), tenantID, created.ID,
		strings.NewReader(validOvpn), int64(len(validOvpn)))
	require.NoError(t, err)
	assert.NotEmpty(t, updated.OvpnConfigPath)
}

func TestVpnService_UploadOvpnConfig_InvalidFile_NoRemote(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, vpnresource.CreateRequest{Name: "VPN"})
	require.NoError(t, err)

	badContent := "dev tun\nproto udp\n" // missing 'remote'
	_, err = svc.UploadOvpnConfig(context.Background(), tenantID, created.ID,
		strings.NewReader(badContent), int64(len(badContent)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remote")
}

func TestVpnService_UploadOvpnConfig_InvalidFile_NoDev(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, vpnresource.CreateRequest{Name: "VPN"})
	require.NoError(t, err)

	badContent := "remote vpn.example.com 1194\nproto udp\n" // missing 'dev'
	_, err = svc.UploadOvpnConfig(context.Background(), tenantID, created.ID,
		strings.NewReader(badContent), int64(len(badContent)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dev")
}

func TestVpnService_UploadOvpnConfig_NotFound(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	_, err := svc.UploadOvpnConfig(context.Background(), tenantID, uuid.New(),
		strings.NewReader(validOvpn), int64(len(validOvpn)))
	require.ErrorIs(t, err, vpnresource.ErrNotFound)
}

// ---- UploadAuthFile ----

func TestVpnService_UploadAuthFile_Success(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, vpnresource.CreateRequest{Name: "VPN"})
	require.NoError(t, err)

	content := "username\npassword\n"
	updated, err := svc.UploadAuthFile(context.Background(), tenantID, created.ID,
		strings.NewReader(content), int64(len(content)))
	require.NoError(t, err)
	assert.NotEmpty(t, updated.AuthFilePath)
}

func TestVpnService_UploadAuthFile_NotFound(t *testing.T) {
	svc := vpnresource.NewService(newMockRepo())
	_, err := svc.UploadAuthFile(context.Background(), tenantID, uuid.New(),
		strings.NewReader("u\np\n"), 4)
	require.ErrorIs(t, err, vpnresource.ErrNotFound)
}
