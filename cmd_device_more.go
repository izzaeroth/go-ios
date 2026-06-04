package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/afc"
	"github.com/danielpaulus/go-ios/ios/amfi"
	"github.com/danielpaulus/go-ios/ios/crashreport"
	"github.com/danielpaulus/go-ios/ios/debugserver"
	"github.com/danielpaulus/go-ios/ios/diagnostics"
	"github.com/danielpaulus/go-ios/ios/fileservice"
	"github.com/danielpaulus/go-ios/ios/house_arrest"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/mcinstall"
	"github.com/danielpaulus/go-ios/ios/testmanagerd"
)

func runEraseCommand(ctx commandContext) {
	force, _ := ctx.Args.Bool("--force")
	if !force {
		slog.Warn("are you sure you want to erase device? (y/n)", "udid", ctx.Device.Properties.SerialNumber)
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		exitIfError("An error occured while reading input", err)
		if !strings.HasPrefix(input, "y") {
			slog.Error("abort")
			return
		}
	}

	exitIfError("failed erasing", mcinstall.Erase(ctx.Device))
	fmt.Print(convertToJSONString("ok"))
}

func runRSDCommand(ctx commandContext) {
	listCommand, _ := ctx.Args.Bool("ls")
	if listCommand {
		services := ctx.Device.Rsd.GetServices()
		if JSONdisabled {
			fmt.Println(services)
		} else {
			b, err := marshalJSON(services)
			exitIfError("failed json conversion", err)
			fmt.Println(string(b))
		}
	}
}

func runMobileGestaltCommand(ctx commandContext) {
	conn, _ := diagnostics.New(ctx.Device)
	keys := ctx.Args["<key>"].([]string)
	plist, _ := ctx.Args.Bool("--plist")
	resp, _ := conn.MobileGestaltQuery(keys)
	if plist {
		fmt.Printf("%s\n", ios.ToPlist(resp))
		return
	}
	jb, _ := marshalJSON(resp)
	fmt.Printf("%s\n", jb)
}

func runDeviceStateCommand(ctx commandContext) {
	listCommand, _ := ctx.Args.Bool("list")
	if listCommand {
		deviceState(ctx.Device, true, false, "", "")
		return
	}
	enable, _ := ctx.Args.Bool("enable")
	profileTypeId, _ := ctx.Args.String("<profileTypeId>")
	profileId, _ := ctx.Args.String("<profileId>")
	deviceState(ctx.Device, false, enable, profileTypeId, profileId)
}

func runWifiCommand(ctx commandContext) {
	ssid, _ := ctx.Args.String("--ssid")
	psw, _ := ctx.Args.String("--password")
	encType, _ := ctx.Args.String("--enc-type")
	remove, _ := ctx.Args.Bool("--remove")

	if encType == "" {
		encType = "WPA"
	}

	if remove {
		exitIfError("failed removing wifi", mcinstall.RemoveWifi(ctx.Device, ssid))
	} else {
		exitIfError("failed preparing wifi", mcinstall.PrepareWifi(ctx.Device, ssid, psw, encType))
	}
	fmt.Print(convertToJSONString("ok"))
}

func runPrepareCommand(ctx commandContext) {
	if createCert, _ := ctx.Args.Bool("create-cert"); createCert {
		cert, err := ios.CreateDERFormattedSupervisionCert()
		exitIfError("failed creating cert", err)
		err = os.WriteFile("supervision-cert.der", cert.CertDER, 0o777)
		slog.Info("supervision-cert.der")
		exitIfError("failed writing cert", err)
		err = os.WriteFile("supervision-cert.pem", cert.CertPEM, 0o777)
		slog.Info("supervision-cert.pem")
		exitIfError("failed writing cert", err)
		err = os.WriteFile("supervision-private-key.key", cert.PrivateKeyDER, 0o777)
		slog.Info("supervision-private-key.key")
		exitIfError("failed writing cert", err)
		err = os.WriteFile("supervision-private-key.pem", cert.PrivateKeyPEM, 0o777)
		slog.Info("supervision-private-key.pem")
		exitIfError("failed writing key", err)
		err = os.WriteFile("supervision-csr.csr", []byte(cert.Csr), 0o777)
		slog.Info("supervision-csr.csr")
		exitIfError("failed writing cert", err)
		slog.Info("Golang does not have good PKCS12 format sadly. If you need a p12 file run this: " +
			"'openssl pkcs12 -export -inkey supervision-private-key.pem -in supervision-cert.pem -out certificate.p12 -password pass:a'")
		return
	}
	if printSkip, _ := ctx.Args.Bool("printskip"); printSkip {
		fmt.Println(convertToJSONString(mcinstall.GetAllSetupSkipOptions()))
		return
	}
	if cloudConfig, _ := ctx.Args.Bool("cloudconfig"); cloudConfig {
		conn, err := mcinstall.New(ctx.Device)
		exitIfError("failed connecting to mcinstall", err)
		defer conn.Close()
		config, err := conn.GetCloudConfiguration()
		exitIfError("failed getting cloud configuration", err)
		fmt.Println(convertToJSONString(config))
		return
	}
	skip := mcinstall.GetAllSetupSkipOptions()
	skipArg := ctx.Args["--skip"].([]string)
	if len(skipArg) > 0 {
		skip = skipArg
	}

	certfile, _ := ctx.Args.String("--certfile")
	orgname, _ := ctx.Args.String("--orgname")
	locale, _ := ctx.Args.String("--locale")
	lang, _ := ctx.Args.String("--lang")
	p12password, _ := ctx.Args.String("--p12password")
	if p12password == "" {
		p12password = os.Getenv("P12_PASSWORD")
	}
	var certBytes []byte
	if certfile != "" {
		rawCertBytes, err := os.ReadFile(certfile)
		exitIfError("failed opening cert file", err)
		if orgname == "" {
			logFatal("--orgname must be specified if certfile for supervision is provided")
		}
		certBytes, err = extractDERCertificate(rawCertBytes, p12password)
		exitIfError("failed to parse supervision certificate", err)
	}
	exitIfError("failed erasing", mcinstall.Prepare(ctx.Device, skip, certBytes, orgname, locale, lang))
	fmt.Print(convertToJSONString("ok"))
}

func runSetWallpaperCommand(ctx commandContext) {
	imagePath, _ := ctx.Args.String("<imagePath>")
	p12file, _ := ctx.Args.String("--p12file")
	p12password, _ := ctx.Args.String("--password")
	if p12password == "" {
		p12password = os.Getenv("P12_PASSWORD")
	}
	screen, _ := ctx.Args.String("--screen")
	if screen == "" {
		screen = "home"
	}
	handleSetWallpaper(ctx.Device, imagePath, screen, p12file, p12password)
}

func runGetWallpaperCommand(ctx commandContext) {
	out, _ := ctx.Args.String("--output")
	if out == "" {
		out = "wallpaper.png"
	}
	handleGetWallpaper(ctx.Device, out)
}

func runGetIconLayoutCommand(ctx commandContext) {
	out, _ := ctx.Args.String("--output")
	handleGetIconLayout(ctx.Device, out)
}

func runSetIconLayoutCommand(ctx commandContext) {
	layoutFile, _ := ctx.Args.String("<layoutFile>")
	handleSetIconLayout(ctx.Device, layoutFile)
}

func runCrashCommand(ctx commandContext) {
	if ls, _ := ctx.Args.Bool("ls"); ls {
		pattern, err := ctx.Args.String("<pattern>")
		if err != nil || pattern == "" {
			pattern = "*"
		}
		files, err := crashreport.ListReports(ctx.Device, pattern)
		exitIfError("failed listing crashreports", err)
		fmt.Println(convertToJSONString(map[string]interface{}{"files": files, "length": len(files)}))
	}
	if cp, _ := ctx.Args.Bool("cp"); cp {
		pattern, _ := ctx.Args.String("<srcpattern>")
		target, _ := ctx.Args.String("<target>")
		slog.Debug("cp", "srcpattern", pattern, "target", target)
		err := crashreport.DownloadReports(ctx.Device, pattern, target)
		exitIfError("failed downloading crashreports", err)
	}
	if rm, _ := ctx.Args.Bool("rm"); rm {
		cwd, _ := ctx.Args.String("<cwd>")
		pattern, _ := ctx.Args.String("<pattern>")
		slog.Debug("rm", "cwd", cwd, "pattern", pattern)
		err := crashreport.RemoveReports(ctx.Device, cwd, pattern)
		exitIfError("failed deleting crashreports", err)
	}
}

func runInstrumentsCommand(ctx commandContext) {
	listenerFunc, closeFunc, err := instruments.ListenAppStateNotifications(ctx.Device)
	if err != nil {
		logFatal("failed listening to app state notifications", "error", err)
	}
	go func() {
		for {
			notification, err := listenerFunc()
			if err != nil {
				slog.Error("listener error", "error", err)
				return
			}
			s, _ := json.Marshal(notification)
			fmt.Println(string(s))
		}
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	err = closeFunc()
	if err != nil {
		slog.Warn("timeout during close", "error", err)
	}
}

func runImageCommand(ctx commandContext) {
	if list, _ := ctx.Args.Bool("list"); list {
		listMountedImages(ctx.Device)
	}

	imagePath, _ := ctx.Args.String("--path")
	auto, _ := ctx.Args.Bool("auto")
	if auto {
		basedir, _ := ctx.Args.String("--basedir")
		if basedir == "" {
			basedir = "./devimages"
		}

		var err error
		imagePath, err = imagemounter.DownloadImageFor(ctx.Device, basedir)
		if err != nil {
			slog.Error("failed downloading image", "basedir", basedir, "udid", ctx.Device.Properties.SerialNumber, "err", err)
			return
		}

		slog.Info("success downloaded image", "basedir", basedir, "udid", ctx.Device.Properties.SerialNumber)
	}

	mount, _ := ctx.Args.Bool("mount")
	if mount || auto {
		err := imagemounter.MountImage(ctx.Device, imagePath)
		if err != nil {
			slog.Error("error mounting image", "image", imagePath, "udid", ctx.Device.Properties.SerialNumber, "err", err)
			return
		}
		slog.Info("success mounting image", "image", imagePath, "udid", ctx.Device.Properties.SerialNumber)
	}

	if unmount, _ := ctx.Args.Bool("unmount"); unmount {
		err := imagemounter.UnmountImage(ctx.Device)
		if err != nil {
			slog.Error("error unmounting image", "udid", ctx.Device.Properties.SerialNumber, "err", err)
			return
		}
		slog.Info("success unmounting image", "udid", ctx.Device.Properties.SerialNumber)
	}
}

func runAssistiveTouchCommand(ctx commandContext) {
	runAccessibilityToggle(ctx, assistiveTouch)
}

func runVoiceOverCommand(ctx commandContext) {
	runAccessibilityToggle(ctx, voiceOver)
}

func runZoomCommand(ctx commandContext) {
	runAccessibilityToggle(ctx, zoomTouch)
}

func runAccessibilityToggle(ctx commandContext, run func(ios.DeviceEntry, string, bool)) {
	force, _ := ctx.Args.Bool("--force")
	for _, operation := range []string{"enable", "disable", "toggle", "get"} {
		if enabled, _ := ctx.Args.Bool(operation); enabled {
			run(ctx.Device, operation, force)
		}
	}
}

func runLockdownCommand(ctx commandContext) {
	if get, _ := ctx.Args.Bool("get"); !get {
		return
	}
	key := ""
	if keyArg := ctx.Args["<key>"]; keyArg != nil {
		if keys, ok := keyArg.([]string); ok && len(keys) > 0 {
			key = keys[0]
		}
	}
	domain, _ := ctx.Args.String("--domain")

	lockdownConnection, err := ios.ConnectLockdownWithSession(ctx.Device)
	exitIfError("failed connecting to lockdown", err)
	defer lockdownConnection.Close()

	if key == "" && domain == "" {
		allValues, err := lockdownConnection.GetValues()
		exitIfError("failed getting lockdown values", err)
		fmt.Println(convertToJSONString(allValues.Value))
	} else if domain != "" {
		value, err := lockdownConnection.GetValueForDomain(key, domain)
		exitIfError(fmt.Sprintf("failed getting value from domain '%s'", domain), err)
		fmt.Println(convertToJSONString(value))
	} else {
		value, err := lockdownConnection.GetValue(key)
		exitIfError(fmt.Sprintf("failed getting lockdown value '%s'", key), err)
		fmt.Println(convertToJSONString(value))
	}
}

func runSetLocationCommand(ctx commandContext) {
	lat, _ := ctx.Args.String("--lat")
	lon, _ := ctx.Args.String("--lon")

	if ctx.Device.SupportsRsd() {
		server, err := instruments.NewLocationSimulationService(ctx.Device)
		exitIfError("failed to create location simulation service:", err)

		startLocationSimulation(server, lat, lon)
		return
	}

	setLocation(ctx.Device, lat, lon)
}

func runSetLocationGPXCommand(ctx commandContext) {
	gpxFilePath, _ := ctx.Args.String("--gpxfilepath")
	setLocationGPX(ctx.Device, gpxFilePath)
}

func runTimeFormatCommand(ctx commandContext) {
	force, _ := ctx.Args.Bool("--force")
	for _, operation := range []string{"24h", "12h", "toggle", "get"} {
		if enabled, _ := ctx.Args.Bool(operation); enabled {
			timeFormat(ctx.Device, operation, force)
		}
	}
}

func runHTTPProxyCommand(ctx commandContext) {
	if removeCommand, _ := ctx.Args.Bool("remove"); removeCommand {
		err := mcinstall.RemoveProxy(ctx.Device)
		exitIfError("failed removing proxy", err)
		slog.Info("success")
		return
	}
	host, _ := ctx.Args.String("<host>")
	port, _ := ctx.Args.String("<port>")
	user, _ := ctx.Args.String("<user>")
	pass, _ := ctx.Args.String("<pass>")
	if pass == "" {
		pass = os.Getenv("PROXY_PASSWORD")
	}
	p12file, _ := ctx.Args.String("--p12file")
	p12password, _ := ctx.Args.String("--password")
	if p12password == "" {
		p12password = os.Getenv("P12_PASSWORD")
	}
	p12bytes, err := os.ReadFile(p12file)
	exitIfError("could not read p12-file", err)

	err = mcinstall.SetHttpProxy(ctx.Device, host, port, user, pass, p12bytes, p12password)
	exitIfError("failed", err)
	slog.Info("success")
}

func runProfileCommand(ctx commandContext) {
	if listCommand, _ := ctx.Args.Bool("list"); listCommand {
		handleProfileList(ctx.Device)
	}
	if add, _ := ctx.Args.Bool("add"); add {
		name, _ := ctx.Args.String("<profileFile>")
		p12file, _ := ctx.Args.String("--p12file")
		p12password, _ := ctx.Args.String("--password")
		if p12password == "" {
			p12password = os.Getenv("P12_PASSWORD")
		}
		if p12file != "" {
			handleProfileAddSupervised(ctx.Device, name, p12file, p12password)
			return
		}
		handleProfileAdd(ctx.Device, name)
	}
	if remove, _ := ctx.Args.Bool("remove"); remove {
		name, _ := ctx.Args.String("<profileName>")
		handleProfileRemove(ctx.Device, name)
	}
}

func runForwardCommand(ctx commandContext) {
	mappings, _ := ctx.Args["--port"].([]string)
	if len(mappings) > 0 {
		startMultiForwarding(ctx.Device, mappings)
		return
	}
	hostPort, _ := ctx.Args.Int("<hostPort>")
	targetPort, _ := ctx.Args.Int("<targetPort>")
	startForwarding(ctx.Device, uint16(hostPort), uint16(targetPort))
}

func runLaunchCommand(ctx commandContext) {
	wait, _ := ctx.Args.Bool("--wait")
	bKillExisting, _ := ctx.Args.Bool("--kill-existing")
	bundleID, _ := ctx.Args.String("<bundleID>")
	if bundleID == "" {
		logFatal("please provide a bundleID")
	}
	pControl, err := instruments.NewProcessControl(ctx.Device)
	exitIfError("processcontrol failed", err)
	opts := map[string]any{}
	if bKillExisting {
		opts["KillExisting"] = 1
	}
	args := toArgs(ctx.Args["--arg"].([]string))
	envs := toEnvs(ctx.Args["--env"].([]string))
	pid, err := pControl.LaunchAppWithArgs(bundleID, args, envs, opts)
	exitIfError("launch app command failed", err)
	slog.Info("Process launched", "pid", pid)
	if wait {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		slog.Info("stop listening to logs", "pid", pid)
	}
}

func runSysmontapCommand(ctx commandContext) {
	printSysmontapStats(ctx.Device)
}

func runMemlimitOffCommand(ctx commandContext) {
	processName, _ := ctx.Args.String("--process")

	pControl, err := instruments.NewProcessControl(ctx.Device)
	exitIfError("processcontrol failed", err)
	defer pControl.Close()

	svc, err := instruments.NewDeviceInfoService(ctx.Device)
	exitIfError("failed opening deviceInfoService for getting process list", err)
	defer svc.Close()

	process, err := svc.ProcessByName(processName)
	exitIfError("process not found", err)
	if process.Pid > 1 {
		disabled, err := pControl.DisableMemoryLimit(process.Pid)
		exitIfError("DisableMemoryLimit failed", err)
		slog.Info("memory limit is off", "process", process.Name, "pid", process.Pid, "disabled", disabled)
	}
}

func runKillCommand(ctx commandContext) {
	var response []installationproxy.AppInfo
	bundleID, _ := ctx.Args.String("<bundleID>")
	processIDint, _ := ctx.Args.Int("--pid")
	processName, _ := ctx.Args.String("--process")

	processID := uint64(processIDint)

	if bundleID == "" && processID == 0 && processName == "" {
		logFatal("please provide a bundleID")
	}
	pControl, err := instruments.NewProcessControl(ctx.Device)
	exitIfError("processcontrol failed", err)
	svc, _ := installationproxy.New(ctx.Device)

	if bundleID != "" {
		response, err = svc.BrowseAllApps()
		exitIfError("browsing apps failed", err)

		for _, app := range response {
			if app.CFBundleIdentifier() == bundleID {
				processName = app.CFBundleExecutable()
				break
			}
		}
		if processName == "" {
			slog.Error("not installed", "bundleID", bundleID)
			os.Exit(1)
			return
		}
	}

	service, err := instruments.NewDeviceInfoService(ctx.Device)
	defer service.Close()
	exitIfError("failed opening deviceInfoService for getting process list", err)
	processList, _ := service.ProcessList()
	for _, p := range processList {
		if (processID > 0 && p.Pid == processID) || (processName != "" && p.Name == processName) {
			err = pControl.KillProcess(p.Pid)
			exitIfError("kill process failed ", err)
			if bundleID != "" {
				slog.Info("killed", "bundleID", bundleID, "pid", p.Pid)
			} else {
				slog.Info("killed", "process", p.Name, "pid", p.Pid)
			}
			return
		}
	}
	if bundleID != "" {
		slog.Error("process not found", "bundleID", bundleID)
	} else if processName != "" {
		slog.Error("process not found", "process", processName)
	} else {
		slog.Error("process not found", "pid", processID)
	}
	os.Exit(1)
}

func runTestCommand(ctx commandContext) {
	bundleID, _ := ctx.Args.String("--bundle-id")
	testRunnerBundleId, _ := ctx.Args.String("--test-runner-bundle-id")
	xctestConfig, _ := ctx.Args.String("--xctest-config")

	testsToRunArg := ctx.Args["--test-to-run"]
	var testsToRun []string
	if testsToRunArg != nil && len(testsToRunArg.([]string)) > 0 {
		testsToRun = testsToRunArg.([]string)
	}

	testsToSkipArg := ctx.Args["--test-to-skip"]
	var testsToSkip []string
	if testsToSkipArg != nil && len(testsToSkipArg.([]string)) > 0 {
		testsToSkip = testsToSkipArg.([]string)
	}

	rawTestlog, rawTestlogErr := ctx.Args.String("--log-output")
	env := splitKeyValuePairs(ctx.Args["--env"].([]string), "=")
	isXCTest, _ := ctx.Args.Bool("--xctest")

	config := testmanagerd.TestConfig{
		BundleId:           bundleID,
		TestRunnerBundleId: testRunnerBundleId,
		XctestConfigName:   xctestConfig,
		Env:                env,
		TestsToRun:         testsToRun,
		TestsToSkip:        testsToSkip,
		XcTest:             isXCTest,
		Device:             ctx.Device,
	}

	if rawTestlogErr == nil {
		var writer *os.File = os.Stdout
		if rawTestlog != "-" {
			file, err := os.Create(rawTestlog)
			exitIfError("Cannot open file "+rawTestlog, err)
			writer = file
		}
		defer writer.Close()

		config.Listener = testmanagerd.NewTestListener(writer, writer, os.TempDir())

		testResults, err := testmanagerd.RunTestWithConfig(context.TODO(), config)
		if err != nil {
			slog.Info("Failed running Xcuitest", "error", err)
		}

		slog.Info("test results", "results", testResults)
	} else {
		config.Listener = testmanagerd.NewTestListener(io.Discard, io.Discard, os.TempDir())
		_, err := testmanagerd.RunTestWithConfig(context.TODO(), config)
		if err != nil {
			slog.Info("Failed running Xcuitest", "error", err)
		}
	}
}

func runXCTestCommand(ctx commandContext) {
	xctestrunFilePath, _ := ctx.Args.String("--xctestrun-file-path")

	rawTestlog, rawTestlogErr := ctx.Args.String("--log-output")

	if rawTestlogErr == nil {
		var writer *os.File = os.Stdout
		if rawTestlog != "-" {
			file, err := os.Create(rawTestlog)
			exitIfError("Cannot open file "+rawTestlog, err)
			writer = file
		}
		defer writer.Close()
		listener := testmanagerd.NewTestListener(writer, writer, os.TempDir())

		testResults, err := testmanagerd.StartXCTestWithConfig(context.TODO(), xctestrunFilePath, ctx.Device, listener)
		if err != nil {
			slog.Info("Failed running Xctest", "error", err)
		}

		slog.Info("test results", "results", testResults)
	} else {
		listener := testmanagerd.NewTestListener(io.Discard, io.Discard, os.TempDir())
		_, err := testmanagerd.StartXCTestWithConfig(context.TODO(), xctestrunFilePath, ctx.Device, listener)
		if err != nil {
			slog.Info("Failed running Xctest", "error", err)
		}
	}
}

func runWDACommand(ctx commandContext) {
	bundleID, _ := ctx.Args.String("--bundleid")
	testbundleID, _ := ctx.Args.String("--testrunnerbundleid")
	xctestconfig, _ := ctx.Args.String("--xctestconfig")
	wdaargs := ctx.Args["--arg"].([]string)
	wdaenv := splitKeyValuePairs(ctx.Args["--env"].([]string), "=")

	if bundleID == "" && testbundleID == "" && xctestconfig == "" {
		slog.Info("no bundle ids specified, falling back to defaults")
		bundleID, testbundleID, xctestconfig = "com.facebook.WebDriverAgentRunner.xctrunner", "com.facebook.WebDriverAgentRunner.xctrunner", "WebDriverAgentRunner.xctest"
	}
	if bundleID == "" || testbundleID == "" || xctestconfig == "" {
		slog.Error("please specify either NONE of bundleid, testbundleid and xctestconfig or ALL of them. At least one was empty.", "bundleid", bundleID, "testbundleid", testbundleID, "xctestconfig", xctestconfig)
		return
	}
	slog.Info("Running wda", "bundleid", bundleID, "testbundleid", testbundleID, "xctestconfig", xctestconfig)

	rawTestlog, rawTestlogErr := ctx.Args.String("--log-output")

	var writer io.Writer

	if rawTestlogErr == nil {
		writerCloser := os.Stdout
		writer = writerCloser
		if rawTestlog != "-" {
			file, err := os.Create(rawTestlog)
			exitIfError("Cannot open file "+rawTestlog, err)
			writer = file
		}
		defer writerCloser.Close()
	} else {
		writer = io.Discard
	}

	errorChannel := make(chan error)
	defer close(errorChannel)
	ctxWDA, stopWda := context.WithCancel(context.Background())
	go func() {
		_, err := testmanagerd.RunTestWithConfig(ctxWDA, testmanagerd.TestConfig{
			BundleId:           bundleID,
			TestRunnerBundleId: testbundleID,
			XctestConfigName:   xctestconfig,
			Env:                wdaenv,
			Args:               wdaargs,
			Device:             ctx.Device,
			Listener:           testmanagerd.NewTestListener(writer, writer, os.TempDir()),
		})
		if err != nil {
			errorChannel <- err
		}
		stopWda()
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errorChannel:
		slog.Error("Failed running WDA", "error", err)
		stopWda()
		os.Exit(1)
	case <-ctxWDA.Done():
		slog.Error("WDA process ended unexpectedly")
		os.Exit(1)
	case signal := <-c:
		slog.Info(fmt.Sprintf("os signal %d received, closing...", signal))
		stopWda()
	}
	slog.Info("Done Closing")
}

func runAXCommand(ctx commandContext) {
	startAx(ctx.Device, ctx.Args)
}

func runResetAXCommand(ctx commandContext) {
	resetAx(ctx.Device)
}

func runDebugCommand(ctx commandContext) {
	appPath, _ := ctx.Args.String("<app_path>")
	if appPath == "" {
		logFatal("parameter bundleid and app_path must be specified")
	}
	stopAtEntry, _ := ctx.Args.Bool("--stop-at-entry")
	exitIfError("debug server failed", debugserver.Start(ctx.Device, appPath, stopAtEntry))
}

func runFileCommand(ctx commandContext) {
	if !ctx.Device.SupportsRsd() {
		exitIfError("file command requires iOS 17+ with tunnel", fmt.Errorf("tunnel not running. Start with: ios tunnel start"))
	}

	bundleID, _ := ctx.Args.String("--app")
	groupID, _ := ctx.Args.String("--app-group")
	useCrash, _ := ctx.Args.Bool("--crash")
	useTemp, _ := ctx.Args.Bool("--temp")

	flagCount := 0
	if bundleID != "" {
		flagCount++
	}
	if groupID != "" {
		flagCount++
	}
	if useCrash {
		flagCount++
	}
	if useTemp {
		flagCount++
	}

	if flagCount > 1 {
		exitIfError("file command", fmt.Errorf("can only specify one of: --app, --app-group, --crash, or --temp"))
	}
	if flagCount == 0 {
		exitIfError("file command", fmt.Errorf("must specify one of: --app=<bundleID>, --app-group=<groupID>, --crash, or --temp"))
	}

	var domain fileservice.Domain
	var identifier string

	if bundleID != "" {
		domain = fileservice.DomainAppDataContainer
		identifier = bundleID
	} else if groupID != "" {
		domain = fileservice.DomainAppGroupDataContainer
		identifier = groupID
	} else if useCrash {
		domain = fileservice.DomainSystemCrashLogs
	} else if useTemp {
		domain = fileservice.DomainTemporary
	}

	conn, err := fileservice.New(ctx.Device, domain, identifier)
	exitIfError("file: failed to connect to file service", err)
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			slog.Error("Failed to close file service connection", "error", closeErr)
		}
	}()

	if ls, _ := ctx.Args.Bool("ls"); ls {
		path, _ := ctx.Args.String("--path")
		if path == "" {
			path = "."
		}

		files, err := conn.ListDirectory(path)
		exitIfError("file ls: failed to list directory", err)

		if !JSONdisabled {
			result := map[string]interface{}{
				"path":  path,
				"files": files,
				"count": len(files),
			}
			fmt.Println(convertToJSONString(result))
		} else {
			fmt.Printf("Files in %s:\n", path)
			for _, file := range files {
				fmt.Printf("  %s\n", file)
			}
			fmt.Printf("\nTotal: %d files\n", len(files))
		}
	}

	if pull, _ := ctx.Args.Bool("pull"); pull {
		remotePath, _ := ctx.Args.String("--remote")
		localPath, _ := ctx.Args.String("--local")

		if remotePath == "" {
			exitIfError("file pull", fmt.Errorf("--remote=<path> is required"))
		}
		if localPath == "" {
			exitIfError("file pull", fmt.Errorf("--local=<path> is required"))
		}

		outputFile, err := os.Create(localPath)
		exitIfError("file pull: failed to create output file", err)
		defer outputFile.Close()

		slog.Info(fmt.Sprintf("Downloading %s to %s...", remotePath, localPath))
		err = conn.PullFile(remotePath, outputFile)
		exitIfError("file pull: failed to download file", err)

		fileInfo, err := outputFile.Stat()
		exitIfError("file pull: failed to get file info", err)
		fileSize := fileInfo.Size()

		if !JSONdisabled {
			result := map[string]interface{}{
				"remote": remotePath,
				"local":  localPath,
				"size":   fileSize,
			}
			fmt.Println(convertToJSONString(result))
		} else {
			slog.Info(fmt.Sprintf("Downloaded %d bytes to %s", fileSize, localPath))
		}
	}

	if push, _ := ctx.Args.Bool("push"); push {
		localPath, _ := ctx.Args.String("--local")
		remotePath, _ := ctx.Args.String("--remote")

		if localPath == "" || remotePath == "" {
			exitIfError("push requires --local and --remote paths", fmt.Errorf("missing required arguments"))
		}

		fileInfo, err := os.Stat(localPath)
		exitIfError("push: failed to stat local file", err)

		permissions := int64(fileInfo.Mode().Perm())
		uid := int64(501)
		gid := int64(501)
		fileSize := fileInfo.Size()

		file, err := os.Open(localPath)
		exitIfError("push: failed to open local file", err)
		defer file.Close()

		slog.Info(fmt.Sprintf("Uploading %s to %s...", localPath, remotePath))
		err = conn.PushFile(remotePath, file, fileSize, permissions, uid, gid)
		exitIfError("push: failed to upload file", err)

		if !JSONdisabled {
			result := map[string]interface{}{
				"remote": remotePath,
				"local":  localPath,
				"size":   fileSize,
			}
			fmt.Println(convertToJSONString(result))
		} else {
			slog.Info(fmt.Sprintf("Uploaded %d bytes to %s", fileSize, remotePath))
		}
	}
}

func runFsyncCommand(ctx commandContext) {
	containerBundleId, _ := ctx.Args.String("--app")
	var afcService *afc.Client
	var err error
	if containerBundleId == "" {
		afcService, err = afc.New(ctx.Device)
	} else {
		afcService, err = house_arrest.New(ctx.Device, containerBundleId)
	}
	exitIfError("fsync: connect afc service failed", err)
	defer afcService.Close()

	if rm, _ := ctx.Args.Bool("rm"); rm {
		path, _ := ctx.Args.String("--path")
		isRecursive, _ := ctx.Args.Bool("--r")
		if isRecursive {
			err = afcService.RemoveAll(path)
		} else {
			err = afcService.Remove(path)
		}
		exitIfError("fsync: remove failed", err)
	}

	if tree, _ := ctx.Args.Bool("tree"); tree {
		path, _ := ctx.Args.String("--path")
		err := afcService.WalkDir(path, func(path string, info afc.FileInfo, err error) error {
			s := strings.Split(path, string(os.PathSeparator))
			_, f := filepath.Split(path)
			prefix := strings.Repeat("|  ", len(s)-1)

			suffix := ""
			if info.Type == afc.S_IFDIR {
				suffix = "/"
			}

			fmt.Printf("%s|-%s%s\n", prefix, f, suffix)
			return nil
		})
		exitIfError("fsync: tree view failed", err)
	}

	if mkdir, _ := ctx.Args.Bool("mkdir"); mkdir {
		path, _ := ctx.Args.String("--path")
		err = afcService.MkDir(path)
		exitIfError("fsync: mkdir failed", err)
	}

	if pull, _ := ctx.Args.Bool("pull"); pull {
		sp, _ := ctx.Args.String("--srcPath")
		dp, _ := ctx.Args.String("--dstPath")
		if dp != "" {
			ret, _ := ios.PathExists(dp)
			if !ret {
				err = os.MkdirAll(dp, os.ModePerm)
				exitIfError("mkdir failed", err)
			}
		}

		dp = path.Join(dp, filepath.Base(sp))
		err = afcService.Pull(sp, dp)
		exitIfError("fsync: pull failed", err)
	}
	if push, _ := ctx.Args.Bool("push"); push {
		sp, _ := ctx.Args.String("--srcPath")
		dp, _ := ctx.Args.String("--dstPath")

		err = afcService.Push(sp, dp)
		exitIfError("fsync: push failed", err)
	}
}

func runDevModeCommand(ctx commandContext) {
	enable, _ := ctx.Args.Bool("enable")
	get, _ := ctx.Args.Bool("get")
	enablePostRestart, _ := ctx.Args.Bool("--enable-post-restart")
	if enable {
		err := amfi.EnableDeveloperMode(ctx.Device, enablePostRestart)
		exitIfError("Failed enabling developer mode", err)
	}

	if get {
		devModeEnabled, _ := imagemounter.IsDevModeEnabled(ctx.Device)
		if JSONdisabled {
			fmt.Printf("Developer mode enabled: %v\n", devModeEnabled)
		} else {
			result := map[string]interface{}{"DeveloperModeEnabled": devModeEnabled}
			fmt.Println(convertToJSONString(result))
		}
	}

	if reveal, _ := ctx.Args.Bool("reveal"); reveal {
		conn, err := amfi.New(ctx.Device)
		exitIfError("Failed connecting to AMFI service", err)
		defer conn.Close()
		err = conn.RevealDevMode()
		exitIfError("Failed revealing developer mode menu", err)
		slog.Info("Developer Mode menu has been revealed on the device. Go to Settings → Privacy & Security → Developer Mode to enable it.")
	}
}
