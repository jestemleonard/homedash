// Homedash Alpine.js Application

document.addEventListener('alpine:init', () => {
    // Page dots indicator for scroll-snap sections
    Alpine.data('pageDots', () => ({
        currentPage: 0,
        pages: [],

        init() {
            const sections = document.querySelectorAll('.snap-section[id^="page-"]');
            this.pages = Array.from(sections).map((_, i) => i);

            const observer = new IntersectionObserver((entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting && entry.intersectionRatio > 0.5) {
                        const id = entry.target.id;
                        const page = parseInt(id.replace('page-', ''), 10);
                        if (!isNaN(page)) this.currentPage = page;
                    }
                });
            }, { threshold: 0.5 });

            sections.forEach(el => observer.observe(el));
        },

        goTo(page) {
            const el = document.getElementById('page-' + page);
            if (el) el.scrollIntoView({ behavior: 'smooth' });
        }
    }));

    Alpine.data('app', () => ({
        // Modal/dropdown states
        settingsOpen: false,
        bookmarksOpen: false,

        // Bookmark editing state
        editingBookmark: null,  // index of bookmark being edited, or 'new' for adding
        bookmarkForm: { name: '', url: '', icon: 'link' },

        // Available icons for bookmarks
        bookmarkIcons: ['link', 'film', 'play', 'code', 'mail', 'globe', 'home', 'book', 'music', 'cloud'],

        // Settings (loaded from localStorage)
        settings: {
            layout: 'list',
            bookmarks: true,
            weather: true,
            theme: 'auto',
            hiddenBookmarks: [],
            customBookmarks: null  // null = use server defaults, array = user customized
        },

        // Server-provided bookmarks (set from template)
        serverBookmarks: [],

        // Initialize
        init() {
            this.loadSettings();
            this.applyTheme();

            // Watch for system theme changes when in auto mode
            window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
                if (this.settings.theme === 'auto') {
                    this.applyTheme();
                }
            });
        },

        // Get bookmarks (custom if set, otherwise server defaults)
        get bookmarks() {
            return this.settings.customBookmarks !== null
                ? this.settings.customBookmarks
                : this.serverBookmarks;
        },

        // Load settings from localStorage
        loadSettings() {
            const saved = localStorage.getItem('homedash-settings');
            if (saved) {
                try {
                    const parsed = JSON.parse(saved);
                    this.settings = { ...this.settings, ...parsed };
                } catch (e) {
                    console.error('Failed to parse settings:', e);
                }
            }
        },

        // Save settings to localStorage
        saveSettings() {
            localStorage.setItem('homedash-settings', JSON.stringify(this.settings));
            this.applyTheme();
        },

        // Apply theme to document
        applyTheme() {
            const root = document.documentElement;
            let isDark = true;

            if (this.settings.theme === 'light') {
                isDark = false;
            } else if (this.settings.theme === 'auto') {
                isDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            }

            if (isDark) {
                root.classList.add('dark');
                root.classList.remove('light');
            } else {
                root.classList.add('light');
                root.classList.remove('dark');
            }
        },

        // Toggle a boolean setting
        toggleSetting(key) {
            this.settings[key] = !this.settings[key];
            this.saveSettings();
        },

        // Set layout
        setLayout(layout) {
            this.settings.layout = layout;
            this.saveSettings();
        },

        // Set theme
        setTheme(theme) {
            this.settings.theme = theme;
            this.saveSettings();
        },

        // Check if a bookmark is visible
        isBookmarkVisible(name) {
            return !this.settings.hiddenBookmarks.includes(name);
        },

        // Toggle individual bookmark visibility
        toggleBookmark(name) {
            const idx = this.settings.hiddenBookmarks.indexOf(name);
            if (idx === -1) {
                this.settings.hiddenBookmarks.push(name);
            } else {
                this.settings.hiddenBookmarks.splice(idx, 1);
            }
            this.saveSettings();
        },

        // Initialize custom bookmarks from server defaults if not yet customized
        ensureCustomBookmarks() {
            if (this.settings.customBookmarks === null) {
                this.settings.customBookmarks = JSON.parse(JSON.stringify(this.serverBookmarks));
            }
        },

        // Start adding a new bookmark
        startAddBookmark() {
            this.ensureCustomBookmarks();
            this.bookmarkForm = { name: '', url: '', icon: 'link' };
            this.editingBookmark = 'new';
        },

        // Start editing a bookmark
        startEditBookmark(index) {
            this.ensureCustomBookmarks();
            const bookmark = this.settings.customBookmarks[index];
            this.bookmarkForm = { ...bookmark };
            this.editingBookmark = index;
        },

        // Save bookmark (add or update)
        saveBookmark() {
            if (!this.bookmarkForm.name || !this.bookmarkForm.url) return;

            // Ensure URL has protocol
            let url = this.bookmarkForm.url;
            if (!url.startsWith('http://') && !url.startsWith('https://')) {
                url = 'https://' + url;
            }

            const bookmark = {
                name: this.bookmarkForm.name,
                url: url,
                icon: this.bookmarkForm.icon
            };

            if (this.editingBookmark === 'new') {
                this.settings.customBookmarks.push(bookmark);
            } else {
                this.settings.customBookmarks[this.editingBookmark] = bookmark;
            }

            this.saveSettings();
            this.cancelEditBookmark();
        },

        // Cancel editing
        cancelEditBookmark() {
            this.editingBookmark = null;
            this.bookmarkForm = { name: '', url: '', icon: 'link' };
        },

        // Delete a bookmark
        deleteBookmark(index) {
            this.ensureCustomBookmarks();
            const name = this.settings.customBookmarks[index].name;
            this.settings.customBookmarks.splice(index, 1);
            // Also remove from hidden list if present
            const hiddenIdx = this.settings.hiddenBookmarks.indexOf(name);
            if (hiddenIdx !== -1) {
                this.settings.hiddenBookmarks.splice(hiddenIdx, 1);
            }
            this.saveSettings();
        },

        // Reset bookmarks to server defaults
        resetBookmarks() {
            this.settings.customBookmarks = null;
            this.settings.hiddenBookmarks = [];
            this.saveSettings();
            this.cancelEditBookmark();
        }
    }));
});
