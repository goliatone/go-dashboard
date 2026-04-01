package dashboard

// Runtime bundles the canonical dashboard service/controller/executor/bootstrap surface.
type Runtime struct {
	Service    *Service
	Controller *Controller
	API        Executor
	Broadcast  *BroadcastHook
}

// RuntimeOptions configures runtime bootstrap.
type RuntimeOptions struct {
	Service    Options
	Controller ControllerOptions
	API        Executor
	Broadcast  *BroadcastHook
}

// NewRuntime builds the canonical dashboard runtime with safe defaults.
func NewRuntime(opts RuntimeOptions) *Runtime {
	broadcast := opts.Broadcast
	if broadcast == nil {
		if existing, ok := opts.Service.RefreshHook.(*BroadcastHook); ok {
			broadcast = existing
		}
	}
	if opts.Service.RefreshHook == nil {
		if broadcast == nil {
			broadcast = NewBroadcastHook()
		}
		opts.Service.RefreshHook = broadcast
	}

	service := NewService(opts.Service)
	controllerOpts := opts.Controller
	if controllerOpts.Service == nil {
		controllerOpts.Service = service
	}
	controller := NewController(controllerOpts)
	api := opts.API
	if api == nil {
		api = NewServiceExecutor(service)
	}

	return &Runtime{
		Service:    service,
		Controller: controller,
		API:        api,
		Broadcast:  broadcast,
	}
}
