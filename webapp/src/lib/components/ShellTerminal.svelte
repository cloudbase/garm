<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import { createShellConnection, type ShellConnection } from '$lib/utils/shell';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import '@xterm/xterm/css/xterm.css';

  export let runnerName: string;
  export let onClose: () => void;

  interface TabSession {
    id: number; // Auto-incremented numeric ID for ordering
    key: string; // The map key (e.g., "shell-1", "shell-2")
    title: string;
    connection: ShellConnection | null;
    terminal: Terminal | null;
    fitAddon: FitAddon | null;
    isInitialized: boolean;
    isConnecting: boolean;
    isConnected: boolean;
    error: string;
    isClosing: boolean;
  }

  let tabsMap = new Map<string, TabSession>(); // Key: "shell-1", "shell-2", etc.
  let activeTabKey: string = ''; // Now uses the map key instead of tab ID
  let tabCounter = 1;
  
  // Reactive computed array for template iteration, ordered by ID
  $: tabs = Array.from(tabsMap.values()).sort((a, b) => a.id - b.id);
  
  // Calculate minimum width based on number of tabs
  $: {
    const tabCount = tabs.filter(t => !t.isClosing).length;
    const tabWidth = 120; // Each tab is min 120px
    const newTabButtonWidth = 50; // New tab button width
    const containerPadding = 40; // Container padding
    const minWidthForTabs = (tabCount * tabWidth) + newTabButtonWidth + containerPadding;
    const absoluteMinWidth = 300; // Never go below 300px
    
    const requiredMinWidth = Math.max(minWidthForTabs, absoluteMinWidth);
    
    if (terminalContainer && !isMaximized) {
      const currentWidth = terminalContainer.offsetWidth;
      if (currentWidth < requiredMinWidth) {
        console.log(`Expanding terminal width from ${currentWidth}px to ${requiredMinWidth}px to fit ${tabCount} tabs`);
        terminalContainer.style.width = `${requiredMinWidth}px`;
        // Trigger terminal resize after width change
        setTimeout(() => {
          fitAllTerminals();
        }, 10);
      }
    }
  }
  
  // Debug: Log tabs array changes
  $: {
    console.log(`Reactive tabs array updated: ${tabs.length} tabs`, 
      tabs.map(t => ({ key: t.key, id: t.id, title: t.title, isClosing: t.isClosing })));
    console.log(`Map has ${tabsMap.size} entries:`, Array.from(tabsMap.keys()));
    console.log(`Filtered tabs for rendering: ${tabs.filter(t => !t.isClosing).length} tabs`, 
      tabs.filter(t => !t.isClosing).map(t => ({ key: t.key, title: t.title })));
  }

  let terminalContainer: HTMLDivElement;
  let isMaximized = false;
  let isResizing = false;
  let isDragging = false;
  let initialMouseX = 0;
  let initialMouseY = 0;
  let initialWidth = 0;
  let initialHeight = 0;
  let initialLeft = 0;
  let initialTop = 0;
  let windowedWidth = 800;
  let windowedHeight = 500;
  let windowedLeft = 0;
  let windowedTop = 0;
  let hasRestoredState = false;
  
  // Store and persist terminal dimensions and container size
  const TERMINAL_DIMENSIONS_KEY = 'garm-terminal-dimensions';
  const TERMINAL_CONTAINER_SIZE_KEY = 'garm-terminal-container-size';
  
  function getStoredDimensions() {
    try {
      const stored = localStorage.getItem(TERMINAL_DIMENSIONS_KEY);
      if (stored) {
        const { cols, rows } = JSON.parse(stored);
        if (cols >= 50 && rows >= 15) {
          return { cols, rows };
        }
      }
    } catch (err) {
      console.warn('Failed to load stored terminal dimensions:', err);
    }
    // Default dimensions if nothing stored or invalid
    return { cols: 107, rows: 29 };
  }
  
  function getStoredContainerSize() {
    try {
      const stored = localStorage.getItem(TERMINAL_CONTAINER_SIZE_KEY);
      if (stored) {
        const { width, height } = JSON.parse(stored);
        if (width >= 300 && height >= 200 && width <= 1400 && height <= 800) {
          return { width, height };
        }
      }
    } catch (err) {
      console.warn('Failed to load stored container size:', err);
    }
    // Default container size if nothing stored or invalid
    return { width: 800, height: 500 };
  }
  
  function saveContainerSize(width: number, height: number) {
    if (width >= 300 && height >= 200 && width <= 1400 && height <= 800) {
      try {
        localStorage.setItem(TERMINAL_CONTAINER_SIZE_KEY, JSON.stringify({ width, height }));
        console.log(`saveContainerSize: Saved ${width}x${height}px to localStorage`);
      } catch (err) {
        console.warn('Failed to save container size:', err);
      }
    } else {
      console.warn(`saveContainerSize: Rejecting invalid size ${width}x${height}`);
    }
  }
  
  function saveDimensions(cols: number, rows: number) {
    // Only save if dimensions are reasonable and different from what's stored
    if (cols >= 50 && rows >= 15 && cols <= 200 && rows <= 60) {
      const current = getStoredDimensions();
      if (current.cols !== cols || current.rows !== rows) {
        try {
          localStorage.setItem(TERMINAL_DIMENSIONS_KEY, JSON.stringify({ cols, rows }));
          console.log(`saveDimensions: Saved ${cols}x${rows} to localStorage (was ${current.cols}x${current.rows})`);
        } catch (err) {
          console.warn('Failed to save terminal dimensions:', err);
        }
      } else {
        console.log(`saveDimensions: Skipping save - ${cols}x${rows} already stored`);
      }
    } else {
      console.warn(`saveDimensions: Rejecting invalid dimensions ${cols}x${rows}`);
    }
  }
  
  // Load stored dimensions
  let { cols: lastGoodCols, rows: lastGoodRows } = getStoredDimensions();
  console.log(`ShellTerminal: Loaded stored dimensions ${lastGoodCols}x${lastGoodRows} from localStorage`);

  // Computed values for active tab using Map
  $: activeTab = tabsMap.get(activeTabKey) || null;
  $: connection = activeTab?.connection || null;
  $: terminal = activeTab?.terminal || null;
  $: isConnecting = activeTab?.isConnecting || false;
  $: isConnected = activeTab?.isConnected || false;
  $: error = activeTab?.error || '';

  // Debug: Log when activeTabKey changes (this should trigger z-index changes)
  $: if (activeTabKey) {
    console.log(`Reactive: activeTabKey changed to ${activeTabKey}, this should bring tab to front`);
  }

  // Reactive focus: Focus the active terminal whenever activeTabKey changes
  $: if (activeTabKey && tabsMap.size > 0) {
    const activeTab = tabsMap.get(activeTabKey);
    if (activeTab?.terminal && activeTab.isInitialized && activeTab.isConnected) {
      console.log(`Reactive focus: Bringing terminal ${activeTabKey} to front`);
      // Use tick() to ensure DOM updates (including z-index changes) have completed
      tick().then(() => {
        if (activeTab.terminal) {
          console.log(`Reactive focus: Focusing terminal ${activeTabKey}`);
          activeTab.terminal.focus();
        }
      });
    }
  }

  function createNewTab(): string {
    const shellKey = `shell-${tabCounter}`;
    const newTab: TabSession = {
      id: tabCounter, // Auto-incremented numeric ID for ordering
      key: shellKey, // The map key
      title: `Shell ${tabCounter}`,
      connection: null,
      terminal: null,
      fitAddon: null,
      isInitialized: false,
      isConnecting: true,
      isConnected: false,
      error: '',
      isClosing: false
    };
    
    tabsMap.set(shellKey, newTab);
    tabsMap = tabsMap; // Force Svelte reactivity
    tabCounter++;
    return shellKey; // Return the map key
  }

  function switchToTab(tabKey: string) {
    if (activeTabKey !== tabKey) {
      console.log(`switchToTab: Switching from ${activeTabKey} to ${tabKey} (z-index stacking)`);
      activeTabKey = tabKey; // This will trigger reactive focus
    }
  }

  // Reactive cleanup - when a tab is marked as closing and is no longer active
  $: {
    // Find closing tabs that are not active
    const closingTabs = Array.from(tabsMap.entries()).filter(([tabKey, tab]) => 
      tab.isClosing && tab.key !== activeTabKey
    );
    
    // Clean up one closing tab per reactive cycle
    if (closingTabs.length > 0) {
      const [tabKey, tab] = closingTabs[0];
      console.log(`Reactive cleanup: Cleaning up tab ${tab.key} that is closing and inactive`);
      cleanupTab(tab);
      // Remove the closing tab from the map
      tabsMap.delete(tabKey);
      // Force reactivity update only once after deletion
      tabsMap = tabsMap;
    }
  }

  function closeTab(tabKey: string) {
    console.log(`closeTab: Closing tab ${tabKey}`);
    const tab = tabsMap.get(tabKey);
    if (!tab) {
      console.error(`closeTab: Tab ${tabKey} not found`);
      return;
    }

    // Before closing, preserve current dimensions (don't let them change during close)
    const currentCols = lastGoodCols;
    const currentRows = lastGoodRows;
    console.log(`closeTab: Preserving dimensions ${currentCols}x${currentRows} during tab close`);
    
    // If this is the active tab, switch to another tab FIRST
    if (activeTabKey === tabKey) {
      if (tabsMap.size > 1) {
        // Find next tab to switch to using ID comparison for ordering
        const allTabs = Array.from(tabsMap.values()).sort((a, b) => a.id - b.id);
        const currentTab = tab;
        
        let newActiveTab: TabSession | undefined;
        
        // Find next tab with higher ID, or previous tab with lower ID
        const nextTab = allTabs.find(t => t.id > currentTab.id);
        if (nextTab) {
          // Found a tab with higher ID
          newActiveTab = nextTab;
        } else {
          // No higher ID, find the highest ID that's lower than current
          const previousTabs = allTabs.filter(t => t.id < currentTab.id).sort((a, b) => b.id - a.id);
          newActiveTab = previousTabs[0];
        }
        
        if (newActiveTab) {
          console.log(`closeTab: Switching from active tab ${tabKey} (ID: ${currentTab.id}) to ${newActiveTab.key} (ID: ${newActiveTab.id}) before cleanup`);
          
          // Switch to new active tab FIRST, before marking as closing
          switchToTab(newActiveTab.key);
          
          // Then mark the tab as closing - this will trigger reactive cleanup
          tab.isClosing = true;
          tabsMap.set(tabKey, tab);
          tabsMap = tabsMap; // Force Svelte reactivity
          
          // Preserve dimensions
          lastGoodCols = currentCols;
          lastGoodRows = currentRows;
          console.log(`closeTab: Preserved dimensions ${lastGoodCols}x${lastGoodRows} for future use`);
        } else {
          // No other tabs left, close the terminal
          console.log(`closeTab: No other tabs left, closing terminal`);
          onClose();
        }
      } else {
        // No other tabs left, close the terminal
        console.log(`closeTab: No other tabs left, closing terminal`);
        onClose();
      }
    } else {
      // This is not the active tab, safe to cleanup immediately
      console.log(`closeTab: Closing inactive tab ${tabKey}`);
      cleanupTab(tab);
      tabsMap.delete(tabKey);
      // Force reactivity update
      tabsMap = tabsMap;
    }
  }

  function cleanupTab(tab: TabSession) {
    console.log(`cleanupTab: Cleaning up tab ${tab.key}`);
    
    // Close connection (this should send the close shell message)
    if (tab.connection) {
      console.log(`cleanupTab: Closing connection for tab ${tab.key}`);
      tab.connection.close();
      tab.connection = null;
    }
    
    // Dispose terminal
    if (tab.terminal) {
      console.log(`cleanupTab: Disposing terminal for tab ${tab.key}`);
      try {
        tab.terminal.dispose();
      } catch (err) {
        console.error(`cleanupTab: Error disposing terminal for tab ${tab.key}:`, err);
      }
      tab.terminal = null;
    }
    
    // Clear other references
    tab.fitAddon = null;
    tab.isInitialized = false;
    tab.isConnected = false;
    tab.isConnecting = false;
  }

  async function createConnection(tabKey: string) {
    const tab = tabsMap.get(tabKey);
    if (!tab) return;

    try {
      const newConnection = await createShellConnection(
        runnerName,
        (data: Uint8Array) => handleData(tabKey, data),
        () => handleReady(tabKey),
        () => handleExit(tabKey),
        (errorMsg: string) => handleError(tabKey, errorMsg)
      );
      
      tab.connection = newConnection;
      tabsMap.set(tabKey, tab);
      tabsMap = tabsMap; // Force Svelte reactivity
    } catch (err) {
      tab.error = err instanceof Error ? err.message : 'Failed to connect';
      tab.isConnecting = false;
      tabsMap.set(tabKey, tab);
      tabsMap = tabsMap; // Force Svelte reactivity
    }
  }

  function initializeTerminalElement(element: HTMLElement, tab: TabSession) {
    // This Svelte action runs when the terminal div is created in DOM
    console.log(`initializeTerminalElement: Action called for ${tab.key}, hasTerminal=${!!tab.terminal}, isInitialized=${tab.isInitialized}`);
    
    if (tab.terminal && !tab.isInitialized) {
      console.log(`initializeTerminalElement: Initializing terminal for ${tab.key} via Svelte action`);
      
      // Create FitAddon if needed
      if (!tab.fitAddon) {
        tab.fitAddon = new FitAddon();
        tab.terminal.loadAddon(tab.fitAddon);
      }

      try {
        // Open terminal in this DOM element
        tab.terminal.open(element);
        console.log(`initializeTerminalElement: Successfully opened terminal ${tab.key} in DOM element`);
      } catch (err) {
        console.error(`initializeTerminalElement: Failed to open terminal ${tab.key}:`, err);
        return;
      }
      
      // Handle terminal input
      tab.terminal.onData((data) => {
        if (tab.connection && tab.isConnected) {
          const encoder = new TextEncoder();
          tab.connection.sendData(encoder.encode(data));
        }
      });

      // Handle terminal resize
      tab.terminal.onResize(({ cols, rows }) => {
        if (tab.connection && tab.isConnected) {
          tab.connection.resize(cols, rows);
        }
      });

      // Mark as initialized
      tab.isInitialized = true;
      tabsMap.set(tab.key, tab);
      tabsMap = tabsMap; // Force Svelte reactivity

      // Focus if active
      if (tab.key === activeTabKey) {
        tab.terminal.focus();
      }

      console.log(`initializeTerminalElement: Terminal ${tab.key} fully initialized and opened in DOM`);
    } else {
      console.log(`initializeTerminalElement: Skipping ${tab.key} - no terminal object or already initialized`);
    }

    return {
      destroy() {
        console.log(`initializeTerminalElement: DOM element being destroyed for ${tab.key}`);
        // DO NOT dispose the terminal here - that should only happen in cleanupTab
        // This is just DOM cleanup
      }
    };
  }


  function fitTerminal() {
    if (!activeTab?.terminal || !activeTab.fitAddon) return;

    activeTab.fitAddon.fit();
    
    if (connection && isConnected) {
      connection.resize(activeTab.terminal.cols, activeTab.terminal.rows);
    }
  }

  function fitTerminalForTab(tab: TabSession) {
    if (!tab.terminal || !tab.connection || !tab.isConnected) {
      console.log(`fitTerminalForTab: Skipping tab ${tab.id} - missing requirements`);
      return;
    }

    // Set terminal to current stored dimensions
    console.log(`fitTerminalForTab: Setting tab ${tab.id} to dimensions ${lastGoodCols}x${lastGoodRows}`);
    tab.terminal.resize(lastGoodCols, lastGoodRows);
    tab.connection.resize(lastGoodCols, lastGoodRows);
  }

  function notifyTerminalResize(tab: TabSession) {
    if (!tab.connection || !tab.isConnected) {
      return;
    }
    
    // Only notify the connection about size change, don't resize terminal buffer
    console.log(`notifyTerminalResize: Notifying tab ${tab.key} about dimensions ${lastGoodCols}x${lastGoodRows}`);
    tab.connection.resize(lastGoodCols, lastGoodRows);
  }

  function fitAllTerminals() {
    // Get dimensions from the active (visible) terminal
    const activeTab = tabsMap.get(activeTabKey);
    if (activeTab?.terminal && activeTab.fitAddon && activeTab.terminal.element) {
      // Fit the active terminal to its container
      activeTab.fitAddon.fit();
      const activeCols = activeTab.terminal.cols;
      const activeRows = activeTab.terminal.rows;
      
      // Only update if we get reasonable dimensions
      if (activeCols >= 50 && activeRows >= 15) {
        lastGoodCols = activeCols;
        lastGoodRows = activeRows;
        // Don't automatically save dimensions from fitAllTerminals - only save on explicit user resize
        console.log(`fitAllTerminals: Active terminal ${activeTab.key} is ${activeCols}x${activeRows}, applying to all tabs`);
        
        // Apply active terminal's size to all terminals and notify all connections
        for (const tab of tabsMap.values()) {
          if (tab.terminal && tab.connection && tab.isConnected) {
            if (tab.key === activeTabKey) {
              // Active terminal - already fitted above, just notify connection
              console.log(`fitAllTerminals: Notifying active terminal ${tab.key} connection about resize to ${lastGoodCols}x${lastGoodRows}`);
              tab.connection.resize(lastGoodCols, lastGoodRows);
            } else {
              // Hidden terminal - resize buffer to match active terminal and notify connection
              console.log(`fitAllTerminals: Resizing hidden terminal ${tab.key} buffer and connection to ${lastGoodCols}x${lastGoodRows}`);
              tab.terminal.resize(lastGoodCols, lastGoodRows);
              tab.connection.resize(lastGoodCols, lastGoodRows);
            }
          }
        }
      } else {
        console.warn(`fitAllTerminals: Active terminal has tiny dimensions ${activeCols}x${activeRows}, skipping update`);
      }
    }
  }

  // Solarized Dark theme
  const solarizedDark = {
    background: '#002b36',
    foreground: '#839496',
    cursor: '#93a1a1',
    black: '#073642',
    red: '#dc322f',
    green: '#859900',
    yellow: '#b58900',
    blue: '#268bd2',
    magenta: '#d33682',
    cyan: '#2aa198',
    white: '#eee8d5',
    brightBlack: '#586e75',
    brightRed: '#cb4b16',
    brightGreen: '#859900',
    brightYellow: '#b58900',
    brightBlue: '#268bd2',
    brightMagenta: '#d33682',
    brightCyan: '#2aa198',
    brightWhite: '#fdf6e3'
  };

  // Light mode friendly theme (dark background that works in light UI)
  const solarizedLight = {
    background: '#2d3748', // Dark gray background for light mode
    foreground: '#e2e8f0', // Light gray text
    cursor: '#cbd5e0',
    black: '#1a202c',
    red: '#e53e3e',
    green: '#38a169',
    yellow: '#d69e2e',
    blue: '#3182ce',
    magenta: '#9f7aea',
    cyan: '#0bc5ea',
    white: '#f7fafc',
    brightBlack: '#4a5568',
    brightRed: '#fc8181',
    brightGreen: '#68d391',
    brightYellow: '#f6e05e',
    brightBlue: '#63b3ed',
    brightMagenta: '#b794f6',
    brightCyan: '#76e4f7',
    brightWhite: '#ffffff'
  };

  function updateTerminalTheme() {
    const isDarkMode = document.documentElement.classList.contains('dark');
    const theme = isDarkMode ? solarizedDark : solarizedLight;
    
    // Update theme for all terminals
    for (const tab of tabsMap.values()) {
      if (tab.terminal) {
        tab.terminal.options.theme = theme;
        // Re-fit terminal after theme change
        if (tab.fitAddon) {
          setTimeout(() => tab.fitAddon?.fit(), 0);
        }
      }
    }
  }

  onMount(() => {
    // 1. Check localStorage for terminal dimensions and container size
    const storedDimensions = getStoredDimensions();
    const storedContainerSize = getStoredContainerSize();
    lastGoodCols = storedDimensions.cols;
    lastGoodRows = storedDimensions.rows;
    console.log(`ShellTerminal onMount: Using dimensions ${lastGoodCols}x${lastGoodRows} and container size ${storedContainerSize.width}x${storedContainerSize.height}`);
    
    // 2. Use stored container size to restore user's preferred window size
    const containerWidth = storedContainerSize.width;
    const containerHeight = storedContainerSize.height;
    
    // 3. Set initial container size to stored values
    if (terminalContainer) {
      terminalContainer.style.width = `${containerWidth}px`;
      terminalContainer.style.height = `${containerHeight}px`;
    }
    
    // Detect initial theme from document class
    const isDarkMode = document.documentElement.classList.contains('dark');
    const theme = isDarkMode ? solarizedDark : solarizedLight;

    // Create the first tab
    const firstTabKey = createNewTab();
    activeTabKey = firstTabKey;

    // Create terminal and fitAddon for first tab
    const newTerminal = new Terminal({
      cursorBlink: true,
      theme,
      fontSize: 13,
      fontFamily: 'Monaco, "Menlo", "Ubuntu Mono", monospace',
      allowTransparency: true
    });

    const newFitAddon = new FitAddon();
    newTerminal.loadAddon(newFitAddon);
    
    const firstTab = tabsMap.get(firstTabKey);
    if (firstTab) {
      firstTab.terminal = newTerminal;
      firstTab.fitAddon = newFitAddon;
      tabsMap.set(firstTabKey, firstTab);
      tabsMap = tabsMap; // Force Svelte reactivity
      
    }

    // Create the connection for first tab
    createConnection(firstTabKey);

    // Handle window resize - only fit the currently visible terminal
    function onWindowResize() {
      clearTimeout(resizeTimeout);
      resizeTimeout = setTimeout(() => {
        fitAllTerminals(); // This now only fits the active terminal
      }, 100);
    }

    // Listen for theme changes by observing the document class
    const observer = new MutationObserver(() => updateTerminalTheme());
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class']
    });

    window.addEventListener('resize', onWindowResize);
    document.addEventListener('fullscreenchange', handleFullscreenChange);
    document.addEventListener('keydown', handleKeyDown);

    return () => {
      window.removeEventListener('resize', onWindowResize);
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
      document.removeEventListener('keydown', handleKeyDown);
      observer.disconnect();
    };
  });

  onDestroy(() => {
    // Clean up all tabs - this will send close messages for each connection
    for (const tab of tabsMap.values()) {
      cleanupTab(tab);
    }
  });

  function handleData(tabKey: string, data: Uint8Array) {
    const tab = tabsMap.get(tabKey);
    if (tab?.terminal) {
      const text = new TextDecoder().decode(data);
      tab.terminal.write(text);
    }
  }

  function handleReady(tabKey: string) {
    const tab = tabsMap.get(tabKey);
    if (!tab) return;

    tab.isConnecting = false;
    tab.isConnected = true;
    tabsMap.set(tabKey, tab);
    tabsMap = tabsMap; // Force Svelte reactivity

    // Fit terminal to the restored container size
    if (tab.connection && tab.terminal && tab.fitAddon) {
      console.log(`handleReady: Terminal ${tabKey} connected, fitting to restored container`);
      
      // Fit the terminal to the restored container size
      tab.fitAddon.fit();
      
      // Get the fitted dimensions and send to server
      const cols = tab.terminal.cols;
      const rows = tab.terminal.rows;
      console.log(`handleReady: Terminal fitted to ${cols}x${rows}, sending to server`);
      tab.connection.resize(cols, rows);
      
      // Update stored dimensions if they're reasonable
      if (cols >= 50 && rows >= 15) {
        lastGoodCols = cols;
        lastGoodRows = rows;
      }
    }
  }

  function handleExit(tabKey: string) {
    const tab = tabsMap.get(tabKey);
    if (!tab) return;

    tab.isConnected = false;
    tabsMap.set(tabKey, tab);
    tabsMap = tabsMap; // Force Svelte reactivity
    
    if (tab.terminal) {
      tab.terminal.write('\r\n[Shell session ended]');
    }
  }

  function handleError(tabKey: string, errorMsg: string) {
    const tab = tabsMap.get(tabKey);
    if (!tab) return;

    tab.error = errorMsg;
    tab.isConnecting = false;
    tab.isConnected = false;
    tabsMap.set(tabKey, tab);
    tabsMap = tabsMap; // Force Svelte reactivity
  }

  // Handle window resize
  let resizeTimeout: NodeJS.Timeout;

  // Remove automatic fitting on tab switch - let it happen only when needed

  // Function to add a new tab
  function addNewTab() {
    // Limit maximum number of tabs to 5
    if (tabsMap.size >= 5) {
      console.log('addNewTab: Maximum number of tabs (5) reached');
      return;
    }
    
    const isDarkMode = document.documentElement.classList.contains('dark');
    const theme = isDarkMode ? solarizedDark : solarizedLight;
    
    const newTabKey = createNewTab();
    
    // Create terminal and fitAddon for new tab
    const newTerminal = new Terminal({
      cursorBlink: true,
      theme,
      fontSize: 13,
      fontFamily: 'Monaco, "Menlo", "Ubuntu Mono", monospace',
      allowTransparency: true
    });

    const newFitAddon = new FitAddon();
    newTerminal.loadAddon(newFitAddon);
    
    const newTab = tabsMap.get(newTabKey);
    if (newTab) {
      newTab.terminal = newTerminal;
      newTab.fitAddon = newFitAddon;
      tabsMap.set(newTabKey, newTab);
      tabsMap = tabsMap; // Force Svelte reactivity
      
    }

    // Switch to new tab and create connection
    switchToTab(newTabKey);
    createConnection(newTabKey);
  }

  function toggleMaximize() {
    if (!isMaximized) {
      // Store windowed dimensions and position before going fullscreen
      saveWindowedState();
      terminalContainer.requestFullscreen?.();
    } else {
      // Apply windowed state immediately before exiting fullscreen
      hasRestoredState = true;
      restoreWindowedState();
      document.exitFullscreen?.();
    }
  }

  function saveWindowedState() {
    const rect = terminalContainer.getBoundingClientRect();
    windowedWidth = rect.width;
    windowedHeight = rect.height;
    windowedLeft = rect.left;
    windowedTop = rect.top;
  }

  function restoreWindowedState() {
    terminalContainer.style.position = 'absolute';
    terminalContainer.style.width = `${windowedWidth}px`;
    terminalContainer.style.height = `${windowedHeight}px`;
    terminalContainer.style.left = `${windowedLeft}px`;
    terminalContainer.style.top = `${windowedTop}px`;
    terminalContainer.style.margin = '0';
    terminalContainer.style.zIndex = '1000';
  }

  // Handle ESC key to preemptively restore state
  function handleKeyDown(event: KeyboardEvent) {
    if (event.key === 'Escape' && isMaximized && !hasRestoredState) {
      // User pressed ESC in fullscreen, preemptively restore state
      hasRestoredState = true;
      restoreWindowedState();
    }
  }

  // Handle fullscreen change events
  function handleFullscreenChange() {
    const wasMaximized = isMaximized;
    isMaximized = !!document.fullscreenElement;

    // Restore windowed state when exiting fullscreen (only if not already restored)
    if (wasMaximized && !isMaximized) {
      if (!hasRestoredState) {
        // ESC key was pressed, restore state immediately
        restoreWindowedState();
      }
      hasRestoredState = false; // Reset flag
      
      // Terminal fit with minimal delay - fit all terminals after fullscreen changes
      setTimeout(() => {
        fitAllTerminals();
      }, 10);
    } else if (!wasMaximized && isMaximized) {
      // Clear positioning when entering fullscreen
      terminalContainer.style.position = '';
      terminalContainer.style.width = '';
      terminalContainer.style.height = '';
      terminalContainer.style.left = '';
      terminalContainer.style.top = '';
      terminalContainer.style.margin = '';
      terminalContainer.style.zIndex = '';
      hasRestoredState = false; // Reset flag
      
      setTimeout(() => {
        fitAllTerminals();
      }, 10);
    }
  }

  // Manual resize functionality
  function startResize(event: MouseEvent) {
    if (isMaximized) return; // Don't allow resize in fullscreen
    
    isResizing = true;
    initialMouseX = event.clientX;
    initialMouseY = event.clientY;
    initialWidth = terminalContainer.offsetWidth;
    initialHeight = terminalContainer.offsetHeight;
    
    event.preventDefault();
    document.addEventListener('mousemove', handleResize);
    document.addEventListener('mouseup', stopResize);
    document.body.style.userSelect = 'none';
    document.body.style.cursor = 'nw-resize';
  }

  function handleResize(event: MouseEvent) {
    if (!isResizing) return;
    
    const deltaX = event.clientX - initialMouseX;
    const deltaY = event.clientY - initialMouseY;
    
    // Calculate minimum width based on current number of tabs
    const tabCount = tabs.filter(t => !t.isClosing).length;
    const tabWidth = 120; // Each tab is min 120px
    const newTabButtonWidth = 50; // New tab button width
    const containerPadding = 40; // Container padding
    const minWidthForTabs = (tabCount * tabWidth) + newTabButtonWidth + containerPadding;
    const dynamicMinWidth = Math.max(minWidthForTabs, 300); // Never go below 300px
    
    // Calculate new dimensions with viewport constraints
    // Leave some padding (32px) from viewport edges for visibility
    const maxWidth = window.innerWidth - 32;
    const maxHeight = window.innerHeight - 32;
    
    const newWidth = Math.max(dynamicMinWidth, Math.min(maxWidth, initialWidth + deltaX));
    const newHeight = Math.max(200, Math.min(maxHeight, initialHeight + deltaY));
    
    terminalContainer.style.width = `${newWidth}px`;
    terminalContainer.style.height = `${newHeight}px`;
    
    // Debounced terminal resize - fit all terminals during manual resize
    clearTimeout(resizeTimeout);
    resizeTimeout = setTimeout(() => {
      fitAllTerminals();
    }, 50);
  }

  function stopResize() {
    isResizing = false;
    document.removeEventListener('mousemove', handleResize);
    document.removeEventListener('mouseup', stopResize);
    document.body.style.userSelect = '';
    document.body.style.cursor = '';
    
    // Save the new windowed state and container size
    if (!isMaximized) {
      saveWindowedState();
      // Save the new container size to localStorage
      const newWidth = terminalContainer.offsetWidth;
      const newHeight = terminalContainer.offsetHeight;
      saveContainerSize(newWidth, newHeight);
    }
    
    // Capture new terminal dimensions from the active terminal after manual resize
    setTimeout(() => {
      const activeTab = tabsMap.get(activeTabKey);
      if (activeTab?.terminal && activeTab.fitAddon && activeTab.terminal.element) {
        // Fit the active terminal to the new container size
        activeTab.fitAddon.fit();
        const newCols = activeTab.terminal.cols;
        const newRows = activeTab.terminal.rows;
        
        if (newCols >= 50 && newRows >= 15) {
          console.log(`stopResize: Saving new dimensions from manual resize: ${newCols}x${newRows}`);
          lastGoodCols = newCols;
          lastGoodRows = newRows;
          saveDimensions(newCols, newRows);
        }
      }
      
      // Now apply the captured dimensions to all terminals
      fitAllTerminals();
    }, 100);
  }

  // Drag-to-move functionality
  function startDrag(event: MouseEvent) {
    if (isMaximized) return; // Don't drag in fullscreen
    
    // Check if click is on a button or other interactive element
    const target = event.target as HTMLElement;
    if (target.tagName === 'BUTTON' || target.closest('button')) {
      return;
    }
    
    isDragging = true;
    initialMouseX = event.clientX;
    initialMouseY = event.clientY;
    
    const rect = terminalContainer.getBoundingClientRect();
    initialLeft = rect.left;
    initialTop = rect.top;
    
    event.preventDefault();
    event.stopPropagation();
    document.addEventListener('mousemove', handleDrag);
    document.addEventListener('mouseup', stopDrag);
    document.body.style.userSelect = 'none';
    document.body.style.cursor = 'move';
    
    // Ensure terminal is positioned absolutely for dragging
    terminalContainer.style.position = 'absolute';
    terminalContainer.style.left = `${initialLeft}px`;
    terminalContainer.style.top = `${initialTop}px`;
    terminalContainer.style.margin = '0';
    terminalContainer.style.zIndex = '1000';
  }

  function handleDrag(event: MouseEvent) {
    if (!isDragging) return;
    
    const deltaX = event.clientX - initialMouseX;
    const deltaY = event.clientY - initialMouseY;
    
    // Calculate new position with viewport constraints
    const newLeft = Math.max(0, Math.min(window.innerWidth - terminalContainer.offsetWidth, initialLeft + deltaX));
    const newTop = Math.max(0, Math.min(window.innerHeight - terminalContainer.offsetHeight, initialTop + deltaY));
    
    terminalContainer.style.left = `${newLeft}px`;
    terminalContainer.style.top = `${newTop}px`;
  }

  function stopDrag() {
    isDragging = false;
    document.removeEventListener('mousemove', handleDrag);
    document.removeEventListener('mouseup', stopDrag);
    document.body.style.userSelect = '';
    document.body.style.cursor = '';
    
    // Save the new windowed state
    if (!isMaximized) {
      saveWindowedState();
    }
  }
</script>

<div class="shell-terminal-container" bind:this={terminalContainer}>
  <div class="shell-header" on:mousedown={startDrag} role="button" tabindex="0" title="Drag to move terminal">
    <div class="flex items-center justify-between">
      <span class="text-sm font-medium text-gray-600 dark:text-gray-300 pointer-events-none">
        Shell - {runnerName}
      </span>
      <div class="flex items-center space-x-2 pointer-events-auto">
        <button
          on:click={toggleMaximize}
          class="w-3 h-3 rounded-full bg-green-500 hover:bg-green-400 transition-colors cursor-pointer flex items-center justify-center pointer-events-auto"
          title={isMaximized ? "Restore" : "Maximize"}
          aria-label={isMaximized ? "Restore" : "Maximize"}
        >
          {#if isMaximized}
            <svg class="w-2 h-2 text-green-900" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M3 4a1 1 0 011-1h4a1 1 0 010 2H6.414l2.293 2.293a1 1 0 11-1.414 1.414L5 6.414V8a1 1 0 01-2 0V4zm9 1a1 1 0 010-2h4a1 1 0 011 1v4a1 1 0 01-2 0V6.414l-2.293 2.293a1 1 0 11-1.414-1.414L13.586 5H12zm-9 7a1 1 0 012 0v1.586l2.293-2.293a1 1 0 111.414 1.414L6.414 15H8a1 1 0 010 2H4a1 1 0 01-1-1v-4zm13-1a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 010-2h1.586l-2.293-2.293a1 1 0 111.414-1.414L15 13.586V12a1 1 0 011-1z" clip-rule="evenodd"></path>
            </svg>
          {:else}
            <svg class="w-2 h-2 text-green-900" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M3 4a1 1 0 011-1h12a1 1 0 011 1v12a1 1 0 01-1 1H4a1 1 0 01-1-1V4zm2 2v8h10V6H5z" clip-rule="evenodd"></path>
            </svg>
          {/if}
        </button>
        <div class="w-3 h-3 rounded-full bg-yellow-500"></div>
        <button
          on:click={onClose}
          class="w-3 h-3 rounded-full bg-red-500 hover:bg-red-400 transition-colors cursor-pointer flex items-center justify-center pointer-events-auto"
          title="Close Shell"
          aria-label="Close Shell"
        >
          <svg class="w-2 h-2 text-red-900" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
          </svg>
        </button>
      </div>
    </div>
  </div>

  <!-- Tab Bar -->
  <div class="tab-bar">
    <div class="tabs-container">
      {#each tabs.filter(t => !t.isClosing) as tab (tab.key)}
        <div 
          class="tab {tab.key === activeTabKey ? 'active' : ''}"
          on:click={() => switchToTab(tab.key)}
          on:keydown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') {
              event.preventDefault();
              switchToTab(tab.key);
            }
          }}
          role="button"
          tabindex="0"
        >
          <span class="tab-title">{tab.title}</span>
          <button 
            class="tab-close"
            on:click|stopPropagation={() => closeTab(tab.key)}
            title="Close tab"
            aria-label="Close tab"
          >
            <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
            </svg>
          </button>
        </div>
      {/each}
      <button 
        class="new-tab-button"
        on:click={addNewTab}
        disabled={tabsMap.size >= 5}
        title={tabsMap.size >= 5 ? "Maximum 5 tabs allowed" : "New tab"}
        aria-label="New tab"
      >
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path>
        </svg>
      </button>
    </div>
  </div>

  <div class="shell-body">
    {#each tabs.filter(t => !t.isClosing) as tab (tab.key)}
      <div 
        class="terminal-tab {tab.key === activeTabKey ? 'active' : ''}" 
        data-tab-key={tab.key}
        use:initializeTerminalElement={tab}
      >
        {#if tab.isConnecting}
          <div class="shell-status connecting">
            <div class="flex items-center justify-center space-x-3">
              <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500"></div>
              <span>Connecting to shell...</span>
            </div>
          </div>
        {:else if tab.error}
          <div class="shell-status error">
            <div class="text-center">
              <div class="text-red-400 mb-2">
                <svg class="w-8 h-8 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                </svg>
              </div>
              <p class="text-red-300">Connection Error</p>
              <p class="text-sm text-red-200 mt-1">{tab.error}</p>
            </div>
          </div>
        {/if}
        <!-- Terminal content will be added here by xterm.js -->
      </div>
    {/each}
  </div>
  
  <!-- Resize handle - only show when not in fullscreen -->
  {#if !isMaximized}
    <div 
      class="resize-handle"
      on:mousedown={startResize}
      role="button"
      tabindex="0"
      title="Resize terminal"
      aria-label="Resize terminal by dragging"
    ></div>
  {/if}
</div>

<style>
  .shell-terminal-container {
    background-color: rgb(255 255 255 / 0.85);
    backdrop-filter: blur(8px);
    border: 1px solid rgb(229 231 235 / 0.5);
    border-radius: 0.5rem;
    overflow: hidden;
    box-shadow: 0 25px 50px -12px rgb(0 0 0 / 0.25);
    width: 800px; /* Set a reasonable initial width */
    min-width: 300px;
    height: 500px;
    min-height: 200px;
    max-height: 90vh;
    max-width: none; /* Remove max-width constraint */
    display: flex;
    flex-direction: column;
    position: relative;
    resize: none; /* Disable default resize to use custom handle */
  }

  .tab-bar {
    background-color: rgb(229 231 235 / 0.6);
    border-bottom: 1px solid rgb(209 213 219 / 0.7);
    padding: 0;
    flex-shrink: 0;
  }

  :global(.dark) .tab-bar {
    background-color: rgb(55 65 81 / 0.6);
    border-bottom: 1px solid rgb(75 85 99 / 0.7);
  }

  .tabs-container {
    display: flex;
    align-items: center;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .tabs-container::-webkit-scrollbar {
    display: none;
  }

  .tab {
    background-color: rgb(243 244 246 / 0.7);
    border-right: 1px solid rgb(209 213 219 / 0.5);
    padding: 0.5rem 0.75rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 120px;
    max-width: 200px;
    user-select: none;
    transition: background-color 0.15s ease;
  }

  .tab:hover {
    background-color: rgb(243 244 246);
  }

  .tab.active {
    background-color: rgb(255 255 255 / 0.9);
    border-bottom: 2px solid rgb(59 130 246);
  }

  :global(.dark) .tab {
    background-color: rgb(31 41 55 / 0.7);
    border-right: 1px solid rgb(55 65 81 / 0.5);
  }

  :global(.dark) .tab:hover {
    background-color: rgb(31 41 55);
  }

  :global(.dark) .tab.active {
    background-color: rgb(30 41 59 / 0.9);
    border-bottom: 2px solid rgb(59 130 246);
  }

  .tab-title {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.875rem;
    color: rgb(75 85 99);
  }

  :global(.dark) .tab-title {
    color: rgb(209 213 219);
  }

  .tab.active .tab-title {
    color: rgb(17 24 39);
    font-weight: 500;
  }

  :global(.dark) .tab.active .tab-title {
    color: rgb(243 244 246);
  }

  .tab-close {
    opacity: 0;
    transition: opacity 0.15s ease;
    padding: 0.125rem;
    border-radius: 0.25rem;
    color: rgb(107 114 128);
  }

  .tab:hover .tab-close,
  .tab.active .tab-close {
    opacity: 1;
  }

  .tab-close:hover {
    background-color: rgb(239 68 68 / 0.1);
    color: rgb(239 68 68);
  }

  :global(.dark) .tab-close {
    color: rgb(156 163 175);
  }

  :global(.dark) .tab-close:hover {
    background-color: rgb(239 68 68 / 0.2);
    color: rgb(248 113 113);
  }

  .new-tab-button {
    background-color: transparent;
    border: none;
    padding: 0.5rem 0.75rem;
    cursor: pointer;
    color: rgb(107 114 128);
    transition: all 0.15s ease;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .new-tab-button:hover {
    background-color: rgb(243 244 246 / 0.7);
    color: rgb(59 130 246);
  }

  :global(.dark) .new-tab-button {
    color: rgb(156 163 175);
  }

  :global(.dark) .new-tab-button:hover {
    background-color: rgb(31 41 55 / 0.7);
    color: rgb(96 165 250);
  }

  .new-tab-button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .new-tab-button:disabled:hover {
    background-color: transparent;
    color: rgb(107 114 128);
  }

  :global(.dark) .new-tab-button:disabled:hover {
    background-color: transparent;
    color: rgb(156 163 175);
  }

  :global(.dark) .shell-terminal-container {
    background-color: rgb(30 41 59 / 0.85);
    border: 1px solid rgb(71 85 105 / 0.3);
  }

  .shell-terminal-container:fullscreen {
    backdrop-filter: none;
    border-radius: 0;
    height: 100vh;
    max-height: none;
    background-color: rgb(255 255 255);
    border: none;
  }

  :global(.dark) .shell-terminal-container:fullscreen {
    background-color: rgb(17 24 39);
  }

  .shell-header {
    background-color: rgb(243 244 246 / 0.9);
    padding: 0.5rem 1rem;
    border-bottom: 1px solid rgb(229 231 235 / 0.7);
    flex-shrink: 0;
    cursor: move;
    user-select: none;
  }

  .shell-header:hover {
    background-color: rgb(243 244 246);
  }

  :global(.dark) .shell-header {
    background-color: rgb(31 41 55 / 0.9);
    border-bottom: 1px solid rgb(55 65 81 / 0.7);
  }

  :global(.dark) .shell-header:hover {
    background-color: rgb(31 41 55);
  }

  .shell-terminal-container:fullscreen .shell-header {
    background-color: rgb(243 244 246);
    border-bottom: 1px solid rgb(229 231 235);
    cursor: default; /* Don't show move cursor in fullscreen */
  }

  :global(.dark) .shell-terminal-container:fullscreen .shell-header {
    background-color: rgb(31 41 55);
    border-bottom: 1px solid rgb(55 65 81);
  }

  .shell-terminal-container:fullscreen .shell-header:hover {
    background-color: rgb(243 244 246); /* No hover effect in fullscreen */
  }

  :global(.dark) .shell-terminal-container:fullscreen .shell-header:hover {
    background-color: rgb(31 41 55); /* No hover effect in fullscreen */
  }

  .shell-body {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    position: relative; /* Contain absolutely positioned terminal tabs */
  }

  .shell-body::-webkit-scrollbar {
    display: none !important;
  }

  .shell-body {
    scrollbar-width: none !important; /* Firefox */
    -ms-overflow-style: none !important; /* IE and Edge */
  }

  .shell-status {
    color: rgb(209 213 219);
    padding: 2rem;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    z-index: 10;
  }

  .shell-status.connecting {
    background-color: rgb(248 250 252); /* Light gray background */
    color: rgb(75 85 99);
  }

  :global(.dark) .shell-status.connecting {
    background-color: rgb(15 23 42); /* Dark background */
    color: rgb(209 213 219);
  }

  .shell-status.error {
    color: rgb(252 165 165);
    background-color: rgb(254 242 242); /* Light red background */
  }

  :global(.dark) .shell-status.error {
    background-color: rgb(35 12 12); /* Dark red background */
  }

  .terminal-tab {
    flex: 1;
    height: 100%;
    padding: 0;
    overflow: hidden;
    border: none !important;
    outline: none !important;
    position: absolute;
    width: 100%;
    top: 0;
    left: 0;
    z-index: 1; /* Background layer for inactive tabs */
    pointer-events: none; /* Prevent interaction when not active */
    visibility: hidden; /* Hide inactive tabs completely */
    opacity: 0; /* Also hide with opacity for smooth transitions */
  }

  .terminal-tab.active {
    z-index: 10; /* Foreground layer for active tab */
    pointer-events: auto; /* Allow interaction when active */
    visibility: visible; /* Show active tab */
    opacity: 1; /* Make active tab fully visible */
  }

  .terminal-tab::-webkit-scrollbar {
    display: none !important;
  }

  .terminal-tab {
    scrollbar-width: none !important; /* Firefox */
    -ms-overflow-style: none !important; /* IE and Edge */
  }

  .resize-handle {
    position: absolute;
    bottom: 0;
    right: 0;
    width: 20px;
    height: 20px;
    cursor: nw-resize;
    background: linear-gradient(
      135deg,
      transparent 0%,
      transparent 30%,
      rgb(156 163 175 / 0.3) 30%,
      rgb(156 163 175 / 0.3) 35%,
      transparent 35%,
      transparent 45%,
      rgb(156 163 175 / 0.3) 45%,
      rgb(156 163 175 / 0.3) 50%,
      transparent 50%,
      transparent 60%,
      rgb(156 163 175 / 0.3) 60%,
      rgb(156 163 175 / 0.3) 65%,
      transparent 65%
    );
    z-index: 10;
    border-bottom-right-radius: 0.5rem;
  }

  .resize-handle:hover {
    background: linear-gradient(
      135deg,
      transparent 0%,
      transparent 30%,
      rgb(156 163 175 / 0.6) 30%,
      rgb(156 163 175 / 0.6) 35%,
      transparent 35%,
      transparent 45%,
      rgb(156 163 175 / 0.6) 45%,
      rgb(156 163 175 / 0.6) 50%,
      transparent 50%,
      transparent 60%,
      rgb(156 163 175 / 0.6) 60%,
      rgb(156 163 175 / 0.6) 65%,
      transparent 65%
    );
  }

  :global(.dark) .resize-handle {
    background: linear-gradient(
      135deg,
      transparent 0%,
      transparent 30%,
      rgb(209 213 219 / 0.3) 30%,
      rgb(209 213 219 / 0.3) 35%,
      transparent 35%,
      transparent 45%,
      rgb(209 213 219 / 0.3) 45%,
      rgb(209 213 219 / 0.3) 50%,
      transparent 50%,
      transparent 60%,
      rgb(209 213 219 / 0.3) 60%,
      rgb(209 213 219 / 0.3) 65%,
      transparent 65%
    );
  }

  :global(.dark) .resize-handle:hover {
    background: linear-gradient(
      135deg,
      transparent 0%,
      transparent 30%,
      rgb(209 213 219 / 0.6) 30%,
      rgb(209 213 219 / 0.6) 35%,
      transparent 35%,
      transparent 45%,
      rgb(209 213 219 / 0.6) 45%,
      rgb(209 213 219 / 0.6) 50%,
      transparent 50%,
      transparent 60%,
      rgb(209 213 219 / 0.6) 60%,
      rgb(209 213 219 / 0.6) 65%,
      transparent 65%
    );
  }

  /* Hide resize handle in fullscreen */
  .shell-terminal-container:fullscreen .resize-handle {
    display: none;
  }
</style>