package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"chakuchuri/backend/internal/audit"
	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/backups"
	"chakuchuri/backend/internal/blueprint"
	"chakuchuri/backend/internal/customers"
	"chakuchuri/backend/internal/directory"
	emailops "chakuchuri/backend/internal/email"
	fileuploads "chakuchuri/backend/internal/files"
	"chakuchuri/backend/internal/foundation"
	"chakuchuri/backend/internal/onboarding"
	"chakuchuri/backend/internal/platform/config"
	"chakuchuri/backend/internal/platform/httpx"
	"chakuchuri/backend/internal/ratesheets"
	"chakuchuri/backend/internal/workflow"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("configuration: %v", err)
	}
	log.Printf("storage paths data=%s files=%s backups=%s", cfg.DataDir, cfg.StorageDir, cfg.BackupDir)
	db := openDatabase(cfg)
	if db != nil {
		defer db.Close()
	}

	recorder := audit.NewRecorder()
	customerService := customers.NewService(recorder)
	authService := auth.NewService(recorder)
	authService.SetCustomerProvisioner(func(user auth.User) {
		customer := customerService.EnsurePortalCustomer(user)
		if customer.ID != "" && customer.ID != user.CustomerID {
			authService.RebindCustomerID(user.ID, customer.ID)
		}
	})
	if db != nil {
		customerService.EnableRepository(customers.NewPostgresRepository(db))
		authService.EnableRepository(auth.NewPostgresRepository(db))
	} else {
		log.Print("CC_ALLOW_JSON_STORE=1: using local JSON repositories for development only.")
		customerService.EnablePersistence(cfg.DataDir)
		authService.EnablePersistence(cfg.DataDir)
	}
	workflowService := workflow.NewService(authService, recorder)
	workflowService.SetWebRTC(cfg.WebRTC())
	directoryService := directory.NewService(authService, recorder)
	emailService := emailops.NewService(authService, recorder)
	rateSheetService := ratesheets.NewService(recorder)
	rateSheetService.SetBookingCreator(func(payload ratesheets.BookingPayload, actor auth.User) (interface{}, error) {
		option := payload.Option
		weight := strconv.FormatFloat(option.RateWeightKG, 'f', -1, 64) + " kg x " + strconv.Itoa(option.Packages) + " / " + payload.PostalPrefix + " / " + strconv.Itoa(option.TotalAmount)
		breakdown, err := json.Marshal(option)
		if err != nil {
			return nil, err
		}
		return workflowService.CreateQuotedShipping(workflow.ShippingRequestPayload{
			CustomerID: actor.CustomerID, Type: "Shipment", Courier: option.Carrier, Service: option.Service,
			Destination: payload.Destination, Zone: option.Zone, Weight: weight,
		}, workflow.ShippingPricing{RateBookID: payload.RateBookID, ServiceID: option.ServiceID, Currency: option.Currency, Amount: option.TotalAmount, Breakdown: breakdown}, actor)
	})
	authService.SetChangeNotifier(workflowService.NotifyExternalChange)
	customerService.SetChangeNotifier(workflowService.NotifyExternalChange)
	authService.SetPresenceNotifier(workflowService.NotifyPresence)
	fileService := fileuploads.NewService(authService, recorder, cfg.StorageDir)
	if db != nil {
		if err := workflowService.EnableRepository(workflow.NewPostgresRepository(db)); err != nil {
			log.Fatalf("workflow initialization: %v", err)
		}
		emailService.EnableRepository(emailops.NewPostgresRepository(db))
		rateSheetService.EnableRepository(ratesheets.NewPostgresRepository(db))
		if err := fileService.EnableRepository(fileuploads.NewPostgresRepository(db)); err != nil {
			log.Fatalf("files initialization: %v", err)
		}
		directoryService.EnableRepository(directory.NewPostgresRepository(db))
	} else {
		if err := workflowService.EnablePersistence(cfg.DataDir); err != nil {
			log.Fatalf("workflow initialization: %v", err)
		}
		emailService.EnablePersistence(cfg.DataDir)
		rateSheetService.EnablePersistence(cfg.DataDir)
		fileService.EnablePersistence(cfg.DataDir)
		directoryService.EnablePersistence(cfg.DataDir)
	}
	for _, pair := range authService.HealCustomerBindings() {
		if changed := workflowService.RemapCustomerID(pair[0], pair[1]); changed > 0 {
			log.Printf("healed drifted customer id %s -> %s (%d workflow rows)", pair[0], pair[1], changed)
		}
	}
	customerService.SetPortalURL(valueOr("PORTAL_URL", cfg.AllowedOrigin))
	customerService.SetDocumentFileValidator(fileService.IsCustomerDocument)
	fileService.SetCustomerDocumentAccess(customerService.CanAccessDocument)
	emailService.SetDeliveryDefaults(strings.TrimRight(valueOr("PORTAL_URL", cfg.AllowedOrigin), "/"), "support@chakuchuri.pk", "")
	emailService.SetRecipientResolver(func(customerID string) (emailops.Recipient, bool) {
		language := workflowService.PlatformSettings().DefaultLanguage
		for _, customer := range customerService.List() {
			if customer.ID != customerID {
				continue
			}
			return emailops.Recipient{
				CustomerID: customer.ID, Name: customer.ContactName, CompanyName: customer.CompanyName,
				Email: customer.Email, Language: language,
			}, true
		}
		return emailops.Recipient{}, false
	})
	workflowService.SetEmailEventPublisher(func(event workflow.EmailEvent) {
		if _, err := emailService.Publish(emailops.BusinessEvent{
			EventKey: event.EventKey, Trigger: event.Trigger, CustomerID: event.CustomerID,
			EntityID: event.EntityID, Data: event.Data,
		}); err != nil {
			log.Printf("email event was not queued trigger=%s entity=%s error=%v", event.Trigger, event.EntityID, err)
		}
	})
	workflowService.SetMessageAttachmentResolver(func(id string, user auth.User) (workflow.MessageAttachment, bool) {
		record, ok := fileService.GetForUser(id, user)
		if !ok {
			return workflow.MessageAttachment{}, false
		}
		return workflow.MessageAttachment{
			FileID: record.ID, Name: record.OriginalName, MimeType: record.MimeType, ByteSize: record.ByteSize,
			URL: record.URL, ThumbnailURL: record.ThumbnailURL,
		}, true
	})
	backupManager := backups.NewManager(backups.Options{
		DataDir:          cfg.DataDir,
		StorageDir:       cfg.StorageDir,
		BackupDir:        cfg.BackupDir,
		BackupOffsiteDir: cfg.BackupOffsiteDir,
		DatabaseURL:      cfg.BackupDatabaseURL,
		Environment:      cfg.Environment,
		Retention:        cfg.BackupRetention,
		Interval:         cfg.BackupInterval,
		OffsiteRequired:  cfg.OffsiteRequired,
		AuditRecorder:    recorder,
	})
	backupContext, stopBackups := context.WithCancel(context.Background())
	defer stopBackups()
	backupManager.Start(backupContext)
	emailService.Start(backupContext)
	workflowService.Start(backupContext)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) { handleReady(w, r, db, cfg) })
	mux.HandleFunc("/api/blueprint", handleBlueprint)
	registerAndroidAppDownload(mux, cfg)
	foundation.Register(mux)
	auth.Register(mux, authService)
	backups.Register(mux, backupManager, authService)
	registerPlatformConnection(mux, authService)
	onboarding.Register(mux, authService, customerService, recorder)
	customers.Register(mux, customerService, authService, fileService)
	directory.Register(mux, directoryService, authService)
	emailops.Register(mux, emailService, authService)
	workflow.Register(mux, workflowService)
	registerRateSheetImport(mux, authService, workflowService, fileService)
	ratesheets.Register(mux, rateSheetService, authService, fileService)
	fileuploads.Register(mux, fileService)
	audit.Register(mux, recorder, func(r *http.Request) (int, string, string, bool) {
		user, ok := authService.UserFromRequest(r)
		if !ok {
			return http.StatusUnauthorized, "not_authenticated", "Sign in is required.", false
		}
		if !auth.HasPermission(user, "audit.read") {
			return http.StatusForbidden, "forbidden", "You do not have access to audit activity.", false
		}
		return http.StatusOK, "", "", true
	})

	handler := httpx.WithRecover(httpx.WithRequestLog(httpx.WithCORS(cfg.AllowedOrigin, mux)))
	server := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       35 * time.Second,
		// Large enough for Android APK download over LAN (~90MB).
		WriteTimeout:   15 * time.Minute,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Printf("ChakuChuri backend listening on http://%s env=%s store=%s", server.Addr, cfg.Environment, cfg.StorageEngine())
	if cfg.BackupOffsiteDir != "" {
		log.Printf("Offsite backups: %s", cfg.BackupOffsiteDir)
	}
	for _, address := range currentPlatformConnectionInfo(cfg.Port).Addresses {
		log.Printf("Phone / LAN API URL: %s", address.APIURL)
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func openDatabase(cfg config.Config) *sql.DB {
	if cfg.DatabaseURL == "" {
		return nil
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		log.Fatalf("database connection failed: %v", err)
	}
	log.Print("PostgreSQL is the live store for auth, customers, workflow, email and files.")
	return db
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}
	info := currentPlatformConnectionInfo(valueOr("PORT", "8002"))
	httpx.Write(w, r, http.StatusOK, map[string]any{
		"status":    "ok",
		"time":      time.Now().UTC().Format(time.RFC3339),
		"apiPort":   info.Port,
		"store":     config.Load().StorageEngine(),
		"addresses": info.Addresses,
	})
}

func handleReady(w http.ResponseWriter, r *http.Request, db *sql.DB, cfg config.Config) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if db == nil {
		if cfg.Production() || !cfg.AllowJSONStore {
			httpx.Write(w, r, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "reason": "database_unavailable"})
			return
		}
	} else if err := db.PingContext(ctx); err != nil {
		httpx.Write(w, r, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "reason": "database_unavailable"})
		return
	}
	for name, path := range map[string]string{"data": cfg.DataDir, "storage": cfg.StorageDir, "backups": cfg.BackupDir} {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			httpx.Write(w, r, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "reason": name + "_directory_unavailable"})
			return
		}
	}
	payload := map[string]any{"status": "ready", "store": cfg.StorageEngine()}
	if db != nil {
		stats := db.Stats()
		payload["database"] = map[string]any{
			"openConnections": stats.OpenConnections,
			"inUse":           stats.InUse,
			"idle":            stats.Idle,
			"waitCount":       stats.WaitCount,
		}
	}
	httpx.Write(w, r, http.StatusOK, payload)
}
func handleBlueprint(w http.ResponseWriter, r *http.Request) {
	if !httpx.RequireMethod(w, r, http.MethodGet) {
		return
	}
	httpx.Write(w, r, http.StatusOK, blueprint.Current())
}

func androidApkPath(cfg config.Config) string {
	if configured := strings.TrimSpace(os.Getenv("ANDROID_APK_PATH")); configured != "" {
		return configured
	}
	// DataDir is <project>/backend/data → project root is two levels up.
	root := filepath.Dir(filepath.Dir(cfg.DataDir))
	return filepath.Join(root, "frontend", "public", "downloads", "chakuchuri-android.apk")
}

func androidBuildMetaPath(apkPath string) string {
	return filepath.Join(filepath.Dir(apkPath), "android-build.json")
}

func registerAndroidAppDownload(mux *http.ServeMux, cfg config.Config) {
	apkPath := androidApkPath(cfg)
	mux.HandleFunc("/api/mobile/android-build", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		info, err := os.Stat(apkPath)
		ready := err == nil && !info.IsDir() && info.Size() > 1024
		payload := map[string]any{
			"ready":       ready,
			"downloadUrl": "/downloads/chakuchuri-android.apk",
			"fileName":    "chakuchuri-android.apk",
		}
		if ready {
			payload["sizeBytes"] = info.Size()
			payload["updatedAt"] = info.ModTime().UTC().Format(time.RFC3339)
		}
		if metaBytes, metaErr := os.ReadFile(androidBuildMetaPath(apkPath)); metaErr == nil {
			var meta map[string]any
			if json.Unmarshal(metaBytes, &meta) == nil {
				for key, value := range meta {
					payload[key] = value
				}
			}
		}
		httpx.Write(w, r, http.StatusOK, payload)
	})
	mux.HandleFunc("/downloads/chakuchuri-android.apk", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET to download the Android APK.")
			return
		}
		info, err := os.Stat(apkPath)
		if err != nil || info.IsDir() {
			httpx.Error(w, r, http.StatusNotFound, "apk_missing", "Android APK is not available on this server yet.")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.android.package-archive")
		w.Header().Set("Content-Disposition", `attachment; filename="chakuchuri-android.apk"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.ServeFile(w, r, apkPath)
	})
	if _, err := os.Stat(apkPath); err == nil {
		log.Printf("Android APK download ready at /downloads/chakuchuri-android.apk (%s)", apkPath)
	} else {
		log.Printf("Android APK not found yet (%s); in-app update will show unavailable until it is placed", apkPath)
	}
}

type platformConnectionAddress struct {
	Interface string `json:"interface"`
	Address   string `json:"address"`
	URL       string `json:"url"`
	APIURL    string `json:"apiUrl"`
}

type platformConnectionInfo struct {
	HostName  string                      `json:"hostName"`
	StableURL string                      `json:"stableUrl"`
	Port      int                         `json:"port"`
	Addresses []platformConnectionAddress `json:"addresses"`
	UpdatedAt string                      `json:"updatedAt"`
}

func registerPlatformConnection(mux *http.ServeMux, authService *auth.Service) {
	mux.HandleFunc("/api/platform/connection", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		if _, ok := authService.UserFromRequest(r); !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		httpx.Write(w, r, http.StatusOK, currentPlatformConnectionInfo(valueOr("PORT", "8002")))
	})
}

func currentPlatformConnectionInfo(apiPort string) platformConnectionInfo {
	if apiPort == "" {
		apiPort = "8002"
	}
	portNumber, err := strconv.Atoi(apiPort)
	if err != nil || portNumber < 1 {
		portNumber = 8002
		apiPort = "8002"
	}

	hostName, _ := os.Hostname()
	hostName = strings.ToLower(strings.TrimSpace(hostName))
	if hostName == "" {
		hostName = "localhost"
	}
	stableHost := hostName
	if !strings.HasSuffix(stableHost, ".local") {
		stableHost += ".local"
	}

	addresses := make([]platformConnectionAddress, 0)
	seen := map[string]bool{}
	interfaces, _ := net.Interfaces()
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		interfaceAddresses, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range interfaceAddresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.IsLoopback() || ip.To4() == nil || (!ip.IsPrivate() && !ip.IsLinkLocalUnicast()) {
				continue
			}
			value := ip.String()
			if seen[value] {
				continue
			}
			seen[value] = true
			addresses = append(addresses, platformConnectionAddress{
				Interface: networkInterface.Name,
				Address:   value,
				URL:       "http://" + value + ":5170",
				APIURL:    "http://" + value + ":" + apiPort,
			})
		}
	}
	sort.Slice(addresses, func(i, j int) bool {
		if addresses[i].Interface == addresses[j].Interface {
			return addresses[i].Address < addresses[j].Address
		}
		return addresses[i].Interface < addresses[j].Interface
	})

	return platformConnectionInfo{
		HostName:  hostName,
		StableURL: "http://" + stableHost + ":" + apiPort,
		Port:      portNumber,
		Addresses: addresses,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func valueOr(key, fallback string) string {
	if found := os.Getenv(key); found != "" {
		return found
	}
	return fallback
}
func registerRateSheetImport(mux *http.ServeMux, authService *auth.Service, workflowService *workflow.Service, fileService *fileuploads.Service) {
	mux.HandleFunc("/api/workflow/ratesheets/import", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		if !auth.HasPermission(user, "shipping.manage") {
			httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to manage shipping rate sheets.")
			return
		}

		const maxRateSheetBytes = 10 << 20
		r.Body = http.MaxBytesReader(w, r.Body, maxRateSheetBytes+1024)
		if err := r.ParseMultipartForm(maxRateSheetBytes); err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_upload", "Rate sheet upload could not be read.")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "file_required", "Choose a CSV or XLSX rate sheet to import.")
			return
		}

		record, err := fileService.Create(file, header, fileuploads.UploadRequest{OwnerType: "rate_sheet"}, user)
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_file", "Only CSV or XLSX rate sheets up to 10 MB are allowed.")
			return
		}
		_, raw, err := fileService.Read(record.ID)
		if err != nil {
			httpx.Error(w, r, http.StatusInternalServerError, "file_read_failed", "Uploaded rate sheet could not be opened.")
			return
		}

		rows, err := ratesheets.Parse(raw, record.OriginalName, r.FormValue("courier"), r.FormValue("service"))
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "ratesheet_parse_failed", "Rate sheet did not contain valid zone, weight and price rows.")
			return
		}

		status := strings.TrimSpace(r.FormValue("status"))
		if status == "" {
			status = "Active"
		}
		payloads := make([]workflow.RateSheetRequest, 0, len(rows))
		for _, row := range rows {
			payloads = append(payloads, workflow.RateSheetRequest{
				Courier:      row.Courier,
				Service:      row.Service,
				Zone:         row.Zone,
				Weight:       row.Weight,
				Price:        row.Price,
				Status:       status,
				SourceFileID: record.ID,
				SourceName:   record.OriginalName,
			})
		}
		saved, err := workflowService.ImportRateSheets(payloads, user)
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "ratesheet_import_failed", "Rate sheet rows could not be saved.")
			return
		}

		httpx.Write(w, r, http.StatusOK, map[string]interface{}{
			"source": record,
			"rates":  saved,
			"count":  len(saved),
		})
	})
}
