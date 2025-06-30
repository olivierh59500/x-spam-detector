package gui

import (
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"x-spam-detector/internal/app"
	"x-spam-detector/internal/models"
)

// MainWindow represents the main application window
type MainWindow struct {
	app         *app.SpamDetectorApp
	window      fyne.Window
	
	// UI Components
	tweetInput    *widget.Entry
	addButton     *widget.Button
	detectButton  *widget.Button
	clearButton   *widget.Button
	
	// Tabs
	tweetsTab     *fyne.Container
	clustersTab   *fyne.Container
	accountsTab   *fyne.Container
	statsTab      *fyne.Container
	
	// Data displays
	tweetsTable   *widget.List
	clustersTable *widget.List
	accountsTable *widget.List
	statsLabels   map[string]*widget.Label
	
	// Settings
	settingsDialog *dialog.FormDialog
	
	// Data bindings
	tweetData    binding.StringList
	clusterData  binding.StringList
	accountData  binding.StringList
	
	// Status
	statusBar     *widget.Label
	progressBar   *widget.ProgressBar
}

// NewMainWindow creates a new main window
func NewMainWindow(spamApp *app.SpamDetectorApp) *MainWindow {
	myApp := fyneapp.New()
	myApp.SetIcon(resourceIconPng) // We'll create this resource
	
	window := myApp.NewWindow("X Spam Detector")
	window.Resize(fyne.NewSize(1200, 800))
	window.SetMaster()
	
	mainWindow := &MainWindow{
		app:    spamApp,
		window: window,
	}
	
	mainWindow.setupUI()
	return mainWindow
}

// setupUI initializes the user interface
func (mw *MainWindow) setupUI() {
	// Initialize data bindings
	mw.tweetData = binding.NewStringList()
	mw.clusterData = binding.NewStringList()
	mw.accountData = binding.NewStringList()
	
	// Create main content
	content := mw.createMainContent()
	
	// Create menu
	menu := mw.createMenu()
	mw.window.SetMainMenu(menu)
	
	// Create status bar
	statusBar := mw.createStatusBar()
	
	// Combine everything
	mainContainer := container.NewBorder(
		nil,          // top
		statusBar,    // bottom
		nil,          // left
		nil,          // right
		content,      // center
	)
	
	mw.window.SetContent(mainContainer)
	
	// Setup keyboard shortcuts
	mw.setupShortcuts()
	
	// Refresh data initially
	mw.refreshData()
}

// createMainContent creates the main content area with tabs
func (mw *MainWindow) createMainContent() *container.AppTabs {
	// Input tab
	inputTab := mw.createInputTab()
	
	// Analysis tabs
	tweetsTab := mw.createTweetsTab()
	clustersTab := mw.createClustersTab()
	accountsTab := mw.createAccountsTab()
	statsTab := mw.createStatsTab()
	
	tabs := container.NewAppTabs(
		container.NewTabItem("Input", inputTab),
		container.NewTabItem("Tweets", tweetsTab),
		container.NewTabItem("Spam Clusters", clustersTab),
		container.NewTabItem("Suspicious Accounts", accountsTab),
		container.NewTabItem("Statistics", statsTab),
	)
	
	return tabs
}

// createInputTab creates the tweet input tab
func (mw *MainWindow) createInputTab() *fyne.Container {
	// Tweet input area
	mw.tweetInput = widget.NewMultiLineEntry()
	mw.tweetInput.SetPlaceHolder("Enter tweet text here or paste multiple tweets (one per line)...")
	mw.tweetInput.Resize(fyne.NewSize(600, 200))
	
	// Buttons
	mw.addButton = widget.NewButton("Add Tweet(s)", mw.onAddTweet)
	mw.addButton.Importance = widget.HighImportance
	
	mw.detectButton = widget.NewButton("Detect Spam", mw.onDetectSpam)
	mw.detectButton.Importance = widget.WarningImportance
	
	mw.clearButton = widget.NewButton("Clear All", mw.onClearAll)
	
	loadSampleButton := widget.NewButton("Load Sample Data", mw.onLoadSample)
	
	buttonContainer := container.NewHBox(
		mw.addButton,
		mw.detectButton,
		mw.clearButton,
		widget.NewSeparator(),
		loadSampleButton,
	)
	
	// Instructions
	instructions := widget.NewRichTextFromMarkdown(`
## How to use X Spam Detector

1. **Add tweets**: Enter tweet text in the input area above. You can paste multiple tweets (one per line).
2. **Detect spam**: Click "Detect Spam" to run the LSH-based spam detection algorithm.
3. **View results**: Switch to other tabs to see detected spam clusters, suspicious accounts, and statistics.

### Sample Data
Click "Load Sample Data" to populate the system with example tweets that demonstrate spam detection capabilities.

### Features
- **MinHash LSH**: Detects near-duplicate content using MinHash signatures
- **SimHash LSH**: Identifies similar content using SimHash fingerprints  
- **Hybrid Mode**: Combines both algorithms for enhanced detection
- **Bot Detection**: Analyzes account patterns to identify suspicious behavior
- **Cluster Analysis**: Groups similar spam tweets and analyzes coordination patterns
`)
	
	return container.NewVBox(
		widget.NewCard("Tweet Input", "", container.NewVBox(
			mw.tweetInput,
			buttonContainer,
		)),
		widget.NewCard("Instructions", "", instructions),
	)
}

// createTweetsTab creates the tweets display tab
func (mw *MainWindow) createTweetsTab() *fyne.Container {
	mw.tweetsTable = widget.NewList(
		func() int {
			count := mw.tweetData.Length()
			return count
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(nil),
				widget.NewLabel("Tweet text will appear here"),
				widget.NewLabel("Author"),
				widget.NewLabel("Status"),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			cont := item.(*fyne.Container)
			icon := cont.Objects[0].(*widget.Icon)
			textLabel := cont.Objects[1].(*widget.Label)
			authorLabel := cont.Objects[2].(*widget.Label)
			statusLabel := cont.Objects[3].(*widget.Label)
			
			tweetText, _ := mw.tweetData.GetValue(id)
			
			// Parse tweet data (this is simplified - in practice you'd have proper data binding)
			tweets := mw.app.GetEngine().GetTweets()
			if len(tweets) > id {
				var tweet *models.Tweet
				i := 0
				for _, t := range tweets {
					if i == id {
						tweet = t
						break
					}
					i++
				}
				
				if tweet != nil {
					if tweet.IsSpam {
						icon.SetResource(resourceWarningIcon)
						statusLabel.SetText("SPAM")
						statusLabel.Importance = widget.DangerImportance
					} else {
						icon.SetResource(resourceCheckIcon)
						statusLabel.SetText("Clean")
						statusLabel.Importance = widget.SuccessImportance
					}
					
					// Truncate long tweets
					text := tweet.Text
					if len(text) > 80 {
						text = text[:77] + "..."
					}
					textLabel.SetText(text)
					authorLabel.SetText("@" + tweet.Author.Username)
				}
			} else {
				textLabel.SetText(tweetText)
				authorLabel.SetText("")
				statusLabel.SetText("")
			}
		},
	)
	
	// Add filter controls
	filterEntry := widget.NewEntry()
	filterEntry.SetPlaceHolder("Filter tweets...")
	
	filterContainer := container.NewHBox(
		widget.NewLabel("Filter:"),
		filterEntry,
		widget.NewButton("Apply", func() {
			// TODO: Implement filtering
			mw.refreshTweets()
		}),
	)
	
	return container.NewBorder(
		filterContainer, // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		mw.tweetsTable, // center
	)
}

// createClustersTab creates the spam clusters display tab
func (mw *MainWindow) createClustersTab() *fyne.Container {
	mw.clustersTable = widget.NewList(
		func() int {
			count := mw.clusterData.Length()
			return count
		},
		func() fyne.CanvasObject {
			return container.NewVBox(
				container.NewHBox(
					widget.NewLabel("Cluster ID"),
					widget.NewLabel("Size: 0"),
					widget.NewLabel("Confidence: 0.0"),
					widget.NewLabel("Level: Medium"),
				),
				widget.NewLabel("Pattern description"),
				widget.NewSeparator(),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			cont := item.(*fyne.Container)
			headerContainer := cont.Objects[0].(*fyne.Container)
			patternLabel := cont.Objects[1].(*widget.Label)
			
			clusterID := headerContainer.Objects[0].(*widget.Label)
			sizeLabel := headerContainer.Objects[1].(*widget.Label)
			confidenceLabel := headerContainer.Objects[2].(*widget.Label)
			levelLabel := headerContainer.Objects[3].(*widget.Label)
			
			clusters := mw.app.GetEngine().GetClusters()
			if len(clusters) > id {
				var cluster *models.SpamCluster
				i := 0
				for _, c := range clusters {
					if i == id {
						cluster = c
						break
					}
					i++
				}
				
				if cluster != nil {
					clusterID.SetText(fmt.Sprintf("Cluster: %s", cluster.ID[:8]))
					sizeLabel.SetText(fmt.Sprintf("Size: %d", cluster.Size))
					confidenceLabel.SetText(fmt.Sprintf("Confidence: %.2f", cluster.Confidence))
					levelLabel.SetText(fmt.Sprintf("Level: %s", cluster.SuspicionLevel))
					patternLabel.SetText(cluster.Pattern)
					
					// Color code by suspicion level
					switch cluster.SuspicionLevel {
					case "critical":
						levelLabel.Importance = widget.DangerImportance
					case "high":
						levelLabel.Importance = widget.WarningImportance
					case "medium":
						levelLabel.Importance = widget.MediumImportance
					default:
						levelLabel.Importance = widget.LowImportance
					}
				}
			}
		},
	)
	
	// Add detailed view button
	selectedClusterIndex := -1
	mw.clustersTable.OnSelected = func(id widget.ListItemID) {
		selectedClusterIndex = id
	}
	
	detailButton := widget.NewButton("View Details", func() {
		if selectedClusterIndex >= 0 {
			mw.showClusterDetails(selectedClusterIndex)
		}
	})
	
	controls := container.NewHBox(
		detailButton,
		widget.NewButton("Export", mw.onExportClusters),
	)
	
	return container.NewBorder(
		controls,        // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		mw.clustersTable, // center
	)
}

// createAccountsTab creates the suspicious accounts display tab
func (mw *MainWindow) createAccountsTab() *fyne.Container {
	mw.accountsTable = widget.NewList(
		func() int {
			return len(mw.app.GetEngine().GetSuspiciousAccounts())
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("@username"),
				widget.NewLabel("Bot Score: 0.0"),
				widget.NewLabel("Spam Tweets: 0"),
				widget.NewLabel("Account Age: 0 days"),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			cont := item.(*fyne.Container)
			usernameLabel := cont.Objects[0].(*widget.Label)
			botScoreLabel := cont.Objects[1].(*widget.Label)
			spamTweetsLabel := cont.Objects[2].(*widget.Label)
			ageLabel := cont.Objects[3].(*widget.Label)
			
			accounts := mw.app.GetEngine().GetSuspiciousAccounts()
			if id < len(accounts) {
				account := accounts[id]
				usernameLabel.SetText("@" + account.Username)
				botScoreLabel.SetText(fmt.Sprintf("Bot Score: %.2f", account.BotScore))
				spamTweetsLabel.SetText(fmt.Sprintf("Spam Tweets: %d", account.SpamTweetCount))
				ageLabel.SetText(fmt.Sprintf("Account Age: %d days", account.AccountAge))
				
				// Color code by bot score
				if account.BotScore >= 0.7 {
					botScoreLabel.Importance = widget.DangerImportance
				} else if account.BotScore >= 0.5 {
					botScoreLabel.Importance = widget.WarningImportance
				}
			}
		},
	)
	
	return container.NewBorder(nil, nil, nil, nil, mw.accountsTable)
}

// createStatsTab creates the statistics display tab
func (mw *MainWindow) createStatsTab() *fyne.Container {
	mw.statsLabels = make(map[string]*widget.Label)
	
	// Create labels for various statistics
	statsLabels := []string{
		"Total Tweets", "Spam Tweets", "Spam Rate", "Active Clusters",
		"Suspicious Accounts", "Processing Time", "Memory Usage",
	}
	
	var statsWidgets []fyne.CanvasObject
	for _, label := range statsLabels {
		valueLabel := widget.NewLabel("0")
		valueLabel.TextStyle = fyne.TextStyle{Bold: true}
		mw.statsLabels[label] = valueLabel
		
		statsWidgets = append(statsWidgets,
			widget.NewCard(label, "", valueLabel),
		)
	}
	
	// Create grid for statistics
	statsGrid := container.NewGridWithColumns(3, statsWidgets...)
	
	// Add performance chart placeholder
	chartLabel := widget.NewLabel("Performance charts would be displayed here in a full implementation")
	chartCard := widget.NewCard("Performance", "", chartLabel)
	
	return container.NewVBox(statsGrid, chartCard)
}

// createMenu creates the application menu
func (mw *MainWindow) createMenu() *fyne.MainMenu {
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Import Tweets", func() {
			dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err == nil && reader != nil {
					// TODO: Implement file import
					reader.Close()
				}
			}, mw.window)
		}),
		fyne.NewMenuItem("Export Results", mw.onExportResults),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			mw.window.Close()
		}),
	)
	
	settingsMenu := fyne.NewMenu("Settings",
		fyne.NewMenuItem("Detection Parameters", mw.showSettingsDialog),
		fyne.NewMenuItem("About", mw.showAboutDialog),
	)
	
	return fyne.NewMainMenu(fileMenu, settingsMenu)
}

// createStatusBar creates the status bar
func (mw *MainWindow) createStatusBar() *fyne.Container {
	mw.statusBar = widget.NewLabel("Ready")
	mw.progressBar = widget.NewProgressBar()
	mw.progressBar.Hide()
	
	return container.NewHBox(
		mw.statusBar,
		widget.NewSeparator(),
		mw.progressBar,
	)
}

// setupShortcuts sets up keyboard shortcuts
func (mw *MainWindow) setupShortcuts() {
	mw.window.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
		switch event.Name {
		case fyne.KeyF5:
			mw.refreshData()
		case fyne.KeyEscape:
			mw.tweetInput.SetText("")
		}
	})
}

// Event handlers

func (mw *MainWindow) onAddTweet() {
	text := mw.tweetInput.Text
	if text == "" {
		dialog.ShowError(fmt.Errorf("please enter tweet text"), mw.window)
		return
	}
	
	mw.setStatus("Adding tweets...")
	mw.showProgress()
	
	go func() {
		defer mw.hideProgress()
		
		// Add tweets (split by lines for multiple tweets)
		err := mw.app.AddTweetsFromText(text)
		if err != nil {
			dialog.ShowError(err, mw.window)
			mw.setStatus("Error adding tweets")
			return
		}
		
		mw.tweetInput.SetText("")
		mw.refreshData()
		mw.setStatus("Tweets added successfully")
	}()
}

func (mw *MainWindow) onDetectSpam() {
	mw.setStatus("Detecting spam...")
	mw.showProgress()
	
	go func() {
		defer mw.hideProgress()
		
		result, err := mw.app.DetectSpam()
		if err != nil {
			dialog.ShowError(err, mw.window)
			mw.setStatus("Error detecting spam")
			return
		}
		
		mw.refreshData()
		mw.setStatus(fmt.Sprintf("Detection complete: %d spam tweets in %d clusters",
			result.SpamTweets, len(result.SpamClusters)))
	}()
}

func (mw *MainWindow) onClearAll() {
	dialog.ShowConfirm("Clear All Data", "Are you sure you want to clear all tweets and results?",
		func(confirmed bool) {
			if confirmed {
				mw.app.ClearAll()
				mw.refreshData()
				mw.setStatus("All data cleared")
			}
		}, mw.window)
}

func (mw *MainWindow) onLoadSample() {
	mw.setStatus("Loading sample data...")
	mw.showProgress()
	
	go func() {
		defer mw.hideProgress()
		
		err := mw.app.LoadSampleData()
		if err != nil {
			dialog.ShowError(err, mw.window)
			mw.setStatus("Error loading sample data")
			return
		}
		
		mw.refreshData()
		mw.setStatus("Sample data loaded successfully")
	}()
}

func (mw *MainWindow) onExportResults() {
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err == nil && writer != nil {
			// TODO: Implement export functionality
			writer.Close()
		}
	}, mw.window)
}

func (mw *MainWindow) onExportClusters() {
	// TODO: Implement cluster export
	dialog.ShowInformation("Export", "Cluster export functionality would be implemented here", mw.window)
}

// Helper methods

func (mw *MainWindow) refreshData() {
	mw.refreshTweets()
	mw.refreshClusters()
	mw.refreshStats()
}

func (mw *MainWindow) refreshTweets() {
	tweets := mw.app.GetEngine().GetTweets()
	
	tweetTexts := make([]string, 0, len(tweets))
	for _, tweet := range tweets {
		status := "Clean"
		if tweet.IsSpam {
			status = "SPAM"
		}
		tweetTexts = append(tweetTexts, fmt.Sprintf("[%s] %s", status, tweet.Text))
	}
	
	mw.tweetData.Set(tweetTexts)
}

func (mw *MainWindow) refreshClusters() {
	clusters := mw.app.GetEngine().GetClusters()
	
	clusterTexts := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		clusterTexts = append(clusterTexts, fmt.Sprintf("Cluster %s: %d tweets, confidence %.2f",
			cluster.ID[:8], cluster.Size, cluster.Confidence))
	}
	
	mw.clusterData.Set(clusterTexts)
}

func (mw *MainWindow) refreshStats() {
	stats := mw.app.GetEngine().GetStatistics()
	
	mw.statsLabels["Total Tweets"].SetText(strconv.FormatInt(stats.TotalTweets, 10))
	mw.statsLabels["Spam Tweets"].SetText(strconv.FormatInt(stats.SpamTweets, 10))
	
	if stats.TotalTweets > 0 {
		spamRate := float64(stats.SpamTweets) / float64(stats.TotalTweets) * 100
		mw.statsLabels["Spam Rate"].SetText(fmt.Sprintf("%.1f%%", spamRate))
	} else {
		mw.statsLabels["Spam Rate"].SetText("0%")
	}
	
	mw.statsLabels["Active Clusters"].SetText(strconv.FormatInt(stats.ActiveClusters, 10))
	mw.statsLabels["Suspicious Accounts"].SetText(strconv.FormatInt(stats.SuspiciousAccounts, 10))
	mw.statsLabels["Processing Time"].SetText(stats.LastProcessingTime.String())
	mw.statsLabels["Memory Usage"].SetText(fmt.Sprintf("%d MB", stats.MemoryUsage/1024/1024))
}

func (mw *MainWindow) setStatus(message string) {
	mw.statusBar.SetText(message)
}

func (mw *MainWindow) showProgress() {
	mw.progressBar.Show()
}

func (mw *MainWindow) hideProgress() {
	mw.progressBar.Hide()
}

func (mw *MainWindow) showClusterDetails(clusterIndex int) {
	clusters := mw.app.GetEngine().GetClusters()
	if clusterIndex >= len(clusters) {
		return
	}
	
	var cluster *models.SpamCluster
	i := 0
	for _, c := range clusters {
		if i == clusterIndex {
			cluster = c
			break
		}
		i++
	}
	
	if cluster == nil {
		return
	}
	
	details := fmt.Sprintf(`**Cluster Details**

**ID:** %s
**Size:** %d tweets
**Confidence:** %.2f
**Suspicion Level:** %s
**Pattern:** %s
**Coordinated:** %t

**Temporal Analysis:**
- First Seen: %s
- Last Seen: %s
- Duration: %s
- Tweets per Minute: %.2f

**Accounts Involved:** %d unique accounts
**Detection Method:** %s
`,
		cluster.ID,
		cluster.Size,
		cluster.Confidence,
		cluster.SuspicionLevel,
		cluster.Pattern,
		cluster.IsCoordinated,
		cluster.FirstSeen.Format(time.RFC3339),
		cluster.LastSeen.Format(time.RFC3339),
		cluster.Duration.String(),
		cluster.TweetsPerMinute,
		cluster.UniqueAccounts,
		cluster.DetectionMethod,
	)
	
	content := widget.NewRichTextFromMarkdown(details)
	content.Wrapping = fyne.TextWrapWord
	
	scroll := container.NewScroll(content)
	scroll.Resize(fyne.NewSize(600, 400))
	
	dialog.ShowCustom("Cluster Details", "Close", scroll, mw.window)
}

func (mw *MainWindow) showSettingsDialog() {
	// TODO: Implement settings dialog
	dialog.ShowInformation("Settings", "Settings dialog would be implemented here", mw.window)
}

func (mw *MainWindow) showAboutDialog() {
	about := `# X Spam Detector

**Version:** 1.0.0

A professional spam detection system for X (Twitter) that uses Locality Sensitive Hashing (LSH) to identify coordinated bot farm activities.

**Features:**
- MinHash LSH for near-duplicate detection
- SimHash LSH for similarity analysis  
- Hybrid detection algorithms
- Bot account analysis
- Coordinated behavior detection
- Real-time clustering

**Developed with:** Go, Fyne UI Framework

**LSH Algorithms:** MinHash, SimHash
`
	
	content := widget.NewRichTextFromMarkdown(about)
	scroll := container.NewScroll(content)
	scroll.Resize(fyne.NewSize(500, 400))
	
	dialog.ShowCustom("About X Spam Detector", "Close", scroll, mw.window)
}

// Show displays the main window
func (mw *MainWindow) Show() {
	mw.window.ShowAndRun()
}

// Placeholder resources - in a real implementation these would be proper icon resources
var (
	resourceIconPng     = fyne.NewStaticResource("icon", []byte{})
	resourceWarningIcon = fyne.NewStaticResource("warning", []byte{})
	resourceCheckIcon   = fyne.NewStaticResource("check", []byte{})
)