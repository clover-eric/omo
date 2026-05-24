package bootstrap

type State string

const (
	StateUninitialized        State = "UNINITIALIZED"
	StatePreflightCheck       State = "PREFLIGHT_CHECK"
	StateAdminCreate          State = "ADMIN_CREATE"
	StateDomainVerify         State = "DOMAIN_VERIFY"
	StateTLSProvision         State = "TLS_PROVISION"
	StatePanelHTTPSEnable     State = "PANEL_HTTPS_ENABLE"
	StateCoreInstall          State = "CORE_INSTALL"
	StateCoreConfigRender     State = "CORE_CONFIG_RENDER"
	StateServiceProfileCreate State = "SERVICE_PROFILE_CREATE"
	StateSubscriptionCreate   State = "SUBSCRIPTION_CREATE"
	StateSecurityHarden       State = "SECURITY_HARDEN"
	StateFinalHealthCheck     State = "FINAL_HEALTH_CHECK"
	StateReady                State = "READY"
)

var OrderedStates = []State{
	StateUninitialized,
	StatePreflightCheck,
	StateAdminCreate,
	StateDomainVerify,
	StateTLSProvision,
	StatePanelHTTPSEnable,
	StateCoreInstall,
	StateCoreConfigRender,
	StateServiceProfileCreate,
	StateSubscriptionCreate,
	StateSecurityHarden,
	StateFinalHealthCheck,
	StateReady,
}
