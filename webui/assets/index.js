// ═══════════════════════════════════════════════════════════════
// SECTION 1: TYPE DEFINITIONS & CONSTANTS
// ═══════════════════════════════════════════════════════════════

/**
 * @typedef {object} PathItem
 * @property {"Dir"|"SymlinkDir"|"File"|"SymlinkFile"} path_type
 * @property {string} name
 * @property {number} mtime
 * @property {number} size
 * @property {number=} resume_position
 */

/**
 * @typedef {object} ServerData
 * @property {string} href
 * @property {string} uri_prefix
 * @property {"Index"|"Edit"|"View"} kind
 * @property {PathItem[]} paths
 * @property {boolean} allow_upload
 * @property {boolean} allow_delete
 * @property {boolean} allow_search
 * @property {boolean} allow_archive
 * @property {boolean} auth
 * @property {string}  user
 * @property {boolean} dir_exists
 * @property {string}  editable
 * @property {boolean} playback_resume_enabled
 * @property {boolean} publink_enabled
 * @property {boolean} publink_is_viewer
 * @property {boolean} enable_data_stats
 * @property {boolean} show_concurrent_uploads_settings
 */

const MAX_UPLOADINGS_KEY    = "dufsMaxUploadings";
const MAX_SUBPATHS_COUNT    = 1000;
const MAX_MULTI_ZIP_SELECTION = 200;
const MAX_MULTI_ZIP_PAYLOAD   = 8192;
const PLAYLIST_KEY            = "sdufs_playlist";

const CHECK_ICON_SUCCESS = `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M13.854 3.646a.5.5 0 0 1 0 .708l-7 7a.5.5 0 0 1-.708 0l-3.5-3.5a.5.5 0 1 1 .708-.708L6.5 10.293l6.646-6.647a.5.5 0 0 1 .708 0z"/></svg>`;

const IFRAME_FORMATS = [".pdf", ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg"];

const VIDEO_FORMATS = [".mp4", ".mov", ".avi", ".wmv", ".flv", ".webm", ".mkv"];
const AUDIO_FORMATS = [".mp3", ".ogg", ".wav", ".m4a"];

const ICONS = {
  dir:         `<svg height="16" viewBox="0 0 14 16" width="14"><path fill-rule="evenodd" d="M13 4H7V3c0-.66-.31-1-1-1H1c-.55 0-1 .45-1 1v10c0 .55.45 1 1 1h12c.55 0 1-.45 1-1V5c0-.55-.45-1-1-1zM6 4H1V3h5v1z"></path></svg>`,
  symlinkFile: `<svg height="16" viewBox="0 0 12 16" width="12"><path fill-rule="evenodd" d="M8.5 1H1c-.55 0-1 .45-1 1v12c0 .55.45 1 1 1h10c.55 0 1-.45 1-1V4.5L8.5 1zM11 14H1V2h7l3 3v9zM6 4.5l4 3-4 3v-2c-.98-.02-1.84.22-2.55.7-.71.48-1.19 1.25-1.45 2.3.02-1.64.39-2.88 1.13-3.73.73-.84 1.69-1.27 2.88-1.27v-2H6z"></path></svg>`,
  symlinkDir:  `<svg height="16" viewBox="0 0 14 16" width="14"><path fill-rule="evenodd" d="M13 4H7V3c0-.66-.31-1-1-1H1c-.55 0-1 .45-1 1v10c0 .55.45 1 1 1h12c.55 0 1-.45 1-1V5c0-.55-.45-1-1-1zM1 3h5v1H1V3zm6 9v-2c-.98-.02-1.84.22-2.55.7-.71.48-1.19 1.25-1.45 2.3.02-1.64.39-2.88 1.13-3.73C4.86 8.43 5.82 8 7.01 8V6l4 3-4 3H7z"></path></svg>`,
  file:        `<svg height="16" viewBox="0 0 12 16" width="12"><path fill-rule="evenodd" d="M6 5H2V4h4v1zM2 8h7V7H2v1zm0 2h7V9H2v1zm0 2h7v-1H2v1zm10-7.5V14c0 .55-.45 1-1 1H1c-.55 0-1-.45-1-1V2c0-.55.45-1 1-1h7.5L12 4.5zM11 5L8 2H1v12h10V5z"></path></svg>`,
  download:    `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M.5 9.9a.5.5 0 0 1 .5.5v2.5a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-2.5a.5.5 0 0 1 1 0v2.5a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2v-2.5a.5.5 0 0 1 .5-.5z"/><path d="M7.646 11.854a.5.5 0 0 0 .708 0l3-3a.5.5 0 0 0-.708-.708L8.5 10.293V1.5a.5.5 0 0 0-1 0v8.793L5.354 8.146a.5.5 0 1 0-.708.708l3 3z"/></svg>`,
  move:        `<svg width="16" height="16" viewBox="0 0 16 16"><path fill-rule="evenodd" d="M1.5 1.5A.5.5 0 0 0 1 2v4.8a2.5 2.5 0 0 0 2.5 2.5h9.793l-3.347 3.346a.5.5 0 0 0 .708.708l4.2-4.2a.5.5 0 0 0 0-.708l-4-4a.5.5 0 0 0-.708.708L13.293 8.3H3.5A1.5 1.5 0 0 1 2 6.8V2a.5.5 0 0 0-.5-.5z"/></svg>`,
  edit:        `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M12.146.146a.5.5 0 0 1 .708 0l3 3a.5.5 0 0 1 0 .708l-10 10a.5.5 0 0 1-.168.11l-5 2a.5.5 0 0 1-.65-.65l2-5a.5.5 0 0 1 .11-.168l10-10zM11.207 2.5 13.5 4.793 14.793 3.5 12.5 1.207 11.207 2.5zm1.586 3L10.5 3.207 4 9.707V10h.5a.5.5 0 0 1 .5.5v.5h.5a.5.5 0 0 1 .5.5v.5h.293l6.5-6.5zm-9.761 5.175-.106.106-1.528 3.821 3.821-1.528.106-.106A.5.5 0 0 1 5 12.5V12h-.5a.5.5 0 0 1-.5-.5V11h-.5a.5.5 0 0 1-.468-.325z"/></svg>`,
  delete:      `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M6.854 7.146a.5.5 0 1 0-.708.708L7.293 9l-1.147 1.146a.5.5 0 0 0 .708.708L8 9.707l1.146 1.147a.5.5 0 0 0 .708-.708L8.707 9l1.147-1.146a.5.5 0 0 0-.708-.708L8 8.293 6.854 7.146z"/><path d="M14 14V4.5L9.5 0H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2zM9.5 3A1.5 1.5 0 0 0 11 4.5h2V14a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1h5.5v2z"/></svg>`,
  view:        `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M4 0a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V2a2 2 0 0 0-2-2zm0 1h8a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1"/></svg>`,
  play:        `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M8 15A7 7 0 1 1 8 1a7 7 0 0 1 0 14zm0 1A8 8 0 1 0 8 0a8 8 0 0 0 0 16z"/><path d="M6.271 5.055a.5.5 0 0 1 .52.038l3.5 2.5a.5.5 0 0 1 0 .814l-3.5 2.5A.5.5 0 0 1 6 10.5v-5a.5.5 0 0 1 .271-.445z"/></svg>`,
  link:           `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M4.715 6.542 3.343 7.914a3 3 0 1 0 4.243 4.243l1.828-1.829A3 3 0 0 0 8.586 5.5L8 6.086a1.002 1.002 0 0 0-.154.199 2 2 0 0 1 .861 3.337L6.88 11.45a2 2 0 1 1-2.83-2.83l.793-.792a4.018 4.018 0 0 1-.128-1.287z"/><path d="M6.586 4.672A3 3 0 0 0 7.414 9.5l.775-.776a2 2 0 0 1-.896-3.346L9.12 3.55a2 2 0 1 1 2.83 2.83l-.793.792c.112.42.155.855.128 1.287l1.372-1.372a3 3 0 1 0-4.243-4.243L6.586 4.672z"/></svg>`,
  prevTrack: `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M3.5 1a.5.5 0 0 1 .5.5V6l7.5-4.5a.5.5 0 0 1 .5-.5.5.5 0 0 1 .5.5v12a.5.5 0 0 1-.5.5.5.5 0 0 1-.5-.5L4 9.5v4.5a.5.5 0 0 1-1 0v-13a.5.5 0 0 1 .5-.5z"/></svg>`,
  nextTrack: `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M12.5 1a.5.5 0 0 0-.5.5V6L4.5 1.5A.5.5 0 0 0 4 1a.5.5 0 0 0-.5.5v12a.5.5 0 0 0 .5.5.5.5 0 0 0 .5-.5L12 9.5v4.5a.5.5 0 0 0 1 0v-13a.5.5 0 0 0-.5-.5z"/></svg>`,
  listIcon: `<svg width="16" height="16" viewBox="0 0 16 16"><path fill-rule="evenodd" d="M2.5 12a.5.5 0 0 1 .5-.5h10a.5.5 0 0 1 0 1H3a.5.5 0 0 1-.5-.5zm0-4a.5.5 0 0 1 .5-.5h10a.5.5 0 0 1 0 1H3a.5.5 0 0 1-.5-.5zm0-4a.5.5 0 0 1 .5-.5h10a.5.5 0 0 1 0 1H3a.5.5 0 0 1-.5-.5z"/></svg>`,
  enterFullscreen: `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M1.5 1h4a.5.5 0 0 1 0 1H2v3.5a.5.5 0 0 1-1 0v-4a.5.5 0 0 1 .5-.5zm9 0h4a.5.5 0 0 1 .5.5v4a.5.5 0 0 1-1 0V2h-3.5a.5.5 0 0 1 0-1zM1 10.5a.5.5 0 0 1 1 0V14h3.5a.5.5 0 0 1 0 1h-4a.5.5 0 0 1-.5-.5v-4zm13 0a.5.5 0 0 1 1 0v4a.5.5 0 0 1-.5.5h-4a.5.5 0 0 1 0-1H14v-3.5z"/></svg>`,
  exitFullscreen: `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M5.5 1a.5.5 0 0 1 .5.5v4a.5.5 0 0 1-.5.5h-4a.5.5 0 0 1 0-1H5V1.5a.5.5 0 0 1 .5-.5zm5 0a.5.5 0 0 1 .5.5V5h3.5a.5.5 0 0 1 0 1h-4a.5.5 0 0 1-.5-.5v-4a.5.5 0 0 1 .5-.5zM1 10.5a.5.5 0 0 1 .5-.5h4a.5.5 0 0 1 .5.5v4a.5.5 0 0 1-1 0V11H1.5a.5.5 0 0 1-.5-.5zm9.5-.5h4a.5.5 0 0 1 0 1H11v3.5a.5.5 0 0 1-1 0v-4a.5.5 0 0 1 .5-.5z"/></svg>`,
  seekBack: `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M2 2.5a.5.5 0 0 0-.5.5v10a.5.5 0 0 0 1 0V3a.5.5 0 0 0-.5-.5z"/><path d="M8 2.5 3.5 8 8 13.5V2.5z"/><path d="M13.5 2.5 9 8l4.5 5.5V2.5z"/></svg>`,
  seekForward: `<svg width="16" height="16" viewBox="0 0 16 16"><path d="M14 2.5a.5.5 0 0 1 .5.5v10a.5.5 0 0 1-1 0V3a.5.5 0 0 1 .5-.5z"/><path d="M8 2.5 12.5 8 8 13.5V2.5z"/><path d="M2.5 2.5 7 8l-4.5 5.5V2.5z"/></svg>`,
};

// ═══════════════════════════════════════════════════════════════
// SECTION 2: APPLICATION STATE
// ═══════════════════════════════════════════════════════════════

/**
 * Centralised application state — all mutable runtime state lives here.
 * @type {{ data: ServerData|null, params: object, selection: Set<number>, maxUploads: number }}
 */
const AppState = {
  /** @type {ServerData|null} */
  data: null,
  /** @type {object} URL search params as plain object */
  params: Object.fromEntries(new URLSearchParams(window.location.search).entries()),
  /** @type {Set<number>} indices into data.paths for selected rows */
  selection: new Set(),
  /** max concurrent uploads (persisted in sessionStorage) */
  maxUploads: 1,
  /** @type {Map<number, Uploader>} uploaders waiting for retry */
  failUploaders: new Map(),
};

// ═══════════════════════════════════════════════════════════════
// SECTION 3: DOM ELEMENT CACHE
// ═══════════════════════════════════════════════════════════════

/**
 * Cached DOM element references — populated once in initDOMCache().
 * JS uses only these references; class selectors are for CSS only.
 */
const UI = {};

function initDOMCache() {
  UI.breadcrumb       = document.getElementById("breadcrumb");

  UI.btnDownload      = document.getElementById("btn-download");
  UI.btnMoveFile      = document.getElementById("btn-move-file");
  UI.btnDeleteFile    = document.getElementById("btn-delete-file");
  UI.btnUploadFile    = document.getElementById("btn-upload-file");
  UI.btnUploadFolder  = document.getElementById("btn-upload-folder");
  UI.inputUploadFile  = document.getElementById("input-upload-file");
  UI.inputUploadFolder = document.getElementById("input-upload-folder");
  UI.btnNewFolder     = document.getElementById("btn-new-folder");
  UI.btnNewFile       = document.getElementById("btn-new-file");
  UI.btnSaveFile      = document.getElementById("btn-save-file");

  UI.searchForm       = document.getElementById("search-form");
  UI.searchInput      = document.getElementById("search-input");

  UI.uploadLimit      = document.getElementById("upload-limit");
  UI.uploadLimitInput = document.getElementById("upload-limit-input");
  UI.uploadLimitApply = document.getElementById("upload-limit-apply");
  UI.uploadLimitBadge = document.getElementById("upload-limit-badge");

  UI.btnLogin         = document.getElementById("btn-login");
  UI.btnLogout        = document.getElementById("btn-logout");
  UI.userName         = document.getElementById("user-name");

  UI.selectionToolbar = document.getElementById("selection-toolbar");
  UI.selectionCount   = document.getElementById("selection-count");
  UI.selBtnDownload   = document.getElementById("sel-btn-download");
  UI.selBtnMove       = document.getElementById("sel-btn-move");
  UI.selBtnDelete     = document.getElementById("sel-btn-delete");
  UI.selBtnPlaylist   = document.getElementById("sel-btn-playlist");
  UI.selBtnResumeReset = document.getElementById("sel-btn-resume-reset");
  UI.selBtnClear      = document.getElementById("sel-btn-clear");

  UI.diskStats        = document.getElementById("disk-stats");
  UI.diskStatsValue   = document.getElementById("disk-stats-value");

  UI.indexPage        = document.getElementById("index-page");
  UI.emptyFolder      = document.getElementById("empty-folder");
  UI.uploadersTable   = document.getElementById("uploaders-table");
  UI.uploadersTbody   = document.getElementById("uploaders-body");
  UI.pathsTable       = document.getElementById("paths-table");
  UI.pathsThead       = document.getElementById("paths-thead");
  UI.pathsTbody       = document.getElementById("paths-tbody");

  UI.editorPage       = document.getElementById("editor-page");
  UI.notEditable      = document.getElementById("not-editable");
  UI.editor           = document.getElementById("editor");

  UI.publinkBackdrop  = document.getElementById("publink-backdrop");
  UI.publinkSheet     = document.getElementById("publink-sheet");
  UI.publinkSheetUrl  = document.getElementById("publink-sheet-url");
  UI.publinkSheetCopy = document.getElementById("publink-sheet-copy");
  UI.publinkSheetCurl = document.getElementById("publink-sheet-curl");
  UI.publinkSheetWget = document.getElementById("publink-sheet-wget");
  UI.publinkSheetPlay = document.getElementById("publink-sheet-play");
}

// ═══════════════════════════════════════════════════════════════
// SECTION 4: ENTRY POINT
// ═══════════════════════════════════════════════════════════════

window.addEventListener("DOMContentLoaded", async () => {
  const $tpl = document.getElementById("index-data");
  if (!$tpl) { alert("No data"); return; }

  AppState.data = JSON.parse(decodeBase64($tpl.innerHTML));

  initDOMCache();
  applyFeatureFlags();

  if (AppState.data.enable_data_stats) {
    // Defer di 200ms per non competere con il primo paint della tabella
    setTimeout(loadDiskStatsAsync, 200);
  }

  addBreadcrumb(AppState.data.href, AppState.data.uri_prefix);

  const { kind } = AppState.data;
  if (kind === "Index") {
    document.title = `Index of ${AppState.data.href} - Dufs`;
    show(UI.indexPage);
    await setupIndexPage();
  } else if (kind === "Edit") {
    document.title = `Edit ${AppState.data.href} - Dufs`;
    show(UI.editorPage);
    await setupEditorPage();
  } else if (kind === "View") {
    document.title = `View ${AppState.data.href} - Dufs`;
    show(UI.editorPage);
    document.body.classList.add("view-mode");
    await setupEditorPage();
  }
});

// ═══════════════════════════════════════════════════════════════
// SECTION 5: FEATURE FLAGS
// ═══════════════════════════════════════════════════════════════

/**
 * Apply server-side feature flags to the DOM in one place.
 * Uses body classes for CSS-driven suppression and show() for JS-driven visibility.
 */
function applyFeatureFlags() {
  const d = AppState.data;

  // CSS body classes — entire feature areas suppressed via CSS rules
  if (!d.publink_enabled)   document.body.classList.add("no-publinks");
}

// ═══════════════════════════════════════════════════════════════
// SECTION 6: PAGE SETUP — INDEX MODE
// ═══════════════════════════════════════════════════════════════

async function setupIndexPage() {
  const d = AppState.data;

  if (d.allow_archive) {
    UI.btnDownload.href = baseUrl() + "?zip";
    UI.btnDownload.title = "Download folder as .zip";
    UI.btnDownload.classList.add("dlwt");
    show(UI.btnDownload);
  }

  if (d.allow_upload) {
    if (d.show_concurrent_uploads_settings) {
      setupUploadLimitControl();
    }
    setupDropzone();
    setupUploadFile();
    setupNewFolder();
    setupNewFile();
  }

  if (d.auth) {
    await setupAuth();
  }

  if (d.allow_search) {
    setupSearch();
  }

  renderPathsTableHead();
  renderPathsTableBody();

  if (d.user) {
    setupDownloadWithToken();
  }

  setupMultiSelection();
  setupPublinkSheet();
  setupResumePositionSync();
}

// ═══════════════════════════════════════════════════════════════
// SECTION 7: PAGE SETUP — EDITOR / VIEW MODE
// ═══════════════════════════════════════════════════════════════

async function setupEditorPage() {
  const d = AppState.data;
  const url = baseUrl();

  // Download button in editor mode only (not in resilient player view)
  if (d.kind !== "View") {
    show(UI.btnDownload);
    UI.btnDownload.href = url;
    UI.btnDownload.removeAttribute("download");
  }

  if (d.auth) {
    await setupAuth();
  }

  if (d.kind === "Edit") {
    if (d.allow_delete && d.allow_upload) {
      show(UI.btnMoveFile);
      UI.btnMoveFile.addEventListener("click", async () => {
        const query = location.href.slice(url.length);
        const newFileUrl = await doMovePath(url);
        if (newFileUrl) location.href = newFileUrl + query;
      });
    }

    if (d.allow_delete) {
      show(UI.btnDeleteFile);
      UI.btnDeleteFile.addEventListener("click", async () => {
        const name = baseName(url);
        await doDeletePath(name, url, () => {
          location.href = location.href.split("/").slice(0, -1).join("/");
        });
      });
    }

    if (d.editable) {
      show(UI.btnSaveFile);
      UI.btnSaveFile.addEventListener("click", saveChange);
    }
  }

  if (!d.editable) {
    const ext  = extName(baseName(url));
    const fileName = baseName(url);

    if (AppState.params.playlist !== undefined) {
      setupPlaylistPlayer(url);
      return;
    }

    if (VIDEO_FORMATS.includes(ext)) {
      const videoUrl = url.split("?")[0];
      UI.notEditable.insertAdjacentHTML("afterend", `
        <div class="video-wrapper">
          <div class="video-toolbar">
            <div class="boost-group">
              <span class="video-toolbar-label">Boost</span>
              <div class="boost-btns">
                <button class="boost-btn active" data-gain="1.0">Off</button>
                <button class="boost-btn" data-gain="1.4">+40%</button>
                <button class="boost-btn" data-gain="1.8">+80%</button>
                <button class="boost-btn" data-gain="2.2">+120%</button>
                <button class="boost-btn" data-gain="2.6">+160%</button>
                <button class="boost-btn" data-gain="3.0">+200%</button>
              </div>
              <button id="resilient-fullscreen" class="fullscreen-btn" type="button"
                title="Enter fitted fullscreen" aria-label="Enter fitted fullscreen"
                aria-controls="resilientVideo" aria-pressed="false">${ICONS.enterFullscreen}</button>
            </div>
          </div>
          <div class="video-toolbar video-toolbar-seek">
            <div class="seek-group">
              <span class="video-toolbar-label">Seek</span>
              <div class="seek-btns">
                <button id="resilient-seek-back" class="seek-btn" type="button"
                  title="Back 30 seconds" aria-label="Back 30 seconds">${ICONS.seekBack}<span>30s</span></button>
                <button id="resilient-seek-forward" class="seek-btn" type="button"
                  title="Forward 30 seconds" aria-label="Forward 30 seconds">${ICONS.seekForward}<span>30s</span></button>
              </div>
            </div>
          </div>
          <div class="video-container">
            <video id="resilientVideo" src="${videoUrl}" controls preload="auto">
              Your browser doesn't support HTML5 video.
            </video>
          </div>
        </div>`);
      setupResilientVideoPlayer();
      return;
    }

    if (AUDIO_FORMATS.includes(ext)) {
      const audioUrl = url.split("?")[0];
      UI.notEditable.insertAdjacentHTML("afterend", `
        <div class="audio-container">
          <h3>${encodedStr(fileName)}</h3>
          <audio id="resilientAudio" controls preload="metadata">
            <source src="${audioUrl}" type="audio/${ext.substring(1)}">
            Your browser doesn't support HTML5 audio.
          </audio>
        </div>`);
      setupResilientAudioPlayer();
      return;
    }

    if (IFRAME_FORMATS.find(v => v === ext)) {
      UI.notEditable.insertAdjacentHTML("afterend",
        `<iframe class="media-iframe" src="${url}"></iframe>`);
      return;
    }

    show(UI.notEditable);
    UI.notEditable.textContent = "Cannot edit because file is too large or binary.";
    return;
  }

  show(UI.editor);
  try {
    const res = await fetch(baseUrl());
    await assertResOK(res);
    const encoding = getEncoding(res.headers.get("content-type"));
    if (encoding === "utf-8") {
      UI.editor.value = await res.text();
    } else {
      const bytes = await res.arrayBuffer();
      UI.editor.value = new TextDecoder(encoding).decode(new DataView(bytes));
    }
  } catch (err) {
    alert(`Failed to get file, ${err.message}`);
  }
}

// ═══════════════════════════════════════════════════════════════
// SECTION 8: TABLE RENDERING
// ═══════════════════════════════════════════════════════════════

function renderPathsTableHead() {
  const headerItems = [
    { name: "name",  props: `colspan="2"`, text: "Name" },
    { name: "mtime", props: ``,            text: "Last Modified" },
    { name: "size",  props: ``,            text: "Size" },
  ];

  const PARAMS = AppState.params;

  const cols = headerItems.map(item => {
    let svg = `<svg width="11" height="11" viewBox="0 0 16 16"><path fill-rule="evenodd" d="M11.5 15a.5.5 0 0 0 .5-.5V2.707l3.146 3.147a.5.5 0 0 0 .708-.708l-4-4a.5.5 0 0 0-.708 0l-4 4a.5.5 0 1 0 .708.708L11 2.707V14.5a.5.5 0 0 0 .5.5zm-7-14a.5.5 0 0 1 .5.5v11.793l3.146-3.147a.5.5 0 0 1 .708.708l-4 4a.5.5 0 0 1-.708 0l-4-4a.5.5 0 0 1 .708-.708L4 13.293V1.5a.5.5 0 0 1 .5-.5z"/></svg>`;
    let order = "desc";
    if (PARAMS.sort === item.name) {
      if (PARAMS.order === "desc") {
        order = "asc";
        svg = `<svg width="11" height="11" viewBox="0 0 16 16"><path fill-rule="evenodd" d="M8 1a.5.5 0 0 1 .5.5v11.793l3.146-3.147a.5.5 0 0 1 .708.708l-4 4a.5.5 0 0 1-.708 0l-4-4a.5.5 0 0 1 .708-.708L7.5 13.293V1.5A.5.5 0 0 1 8 1z"/></svg>`;
      } else {
        svg = `<svg width="11" height="11" viewBox="0 0 16 16"><path fill-rule="evenodd" d="M8 15a.5.5 0 0 0 .5-.5V2.707l3.146 3.147a.5.5 0 0 0 .708-.708l-4-4a.5.5 0 0 0-.708 0l-4 4a.5.5 0 1 0 .708.708L7.5 2.707V14.5a.5.5 0 0 0 .5.5z"/></svg>`;
      }
    }
    const qs = new URLSearchParams({ ...PARAMS, order, sort: item.name }).toString();
    return `<th class="col-${item.name}" ${item.props}><a href="?${qs}">${item.text}<span>${svg}</span></a></th>`;
  }).join("\n");

  const publinks = AppState.data.publink_enabled
    ? `<th class="col-publink">Public Link</th><th class="col-publink-curl"></th><th class="col-publink-wget"></th>`
    : "";
  const resumeCol = AppState.data.playback_resume_enabled
    ? `<th class="col-resume">Resume</th>`
    : "";

  UI.pathsThead.insertAdjacentHTML("beforeend", `
    <tr>
      <th class="col-checkbox"><input type="checkbox" id="select-all-checkbox" title="Select all"></th>
      ${cols}
      ${resumeCol}
      <th class="col-actions">Actions</th>
      ${publinks}
    </tr>`);
}

function renderPathsTableBody() {
  if (AppState.data.paths && AppState.data.paths.length > 0) {
    show(UI.pathsTable);
    // Genera tutto l'HTML in un'unica stringa e inserisce con un solo insertAdjacentHTML
    // per evitare un reflow/repaint per ogni riga (ottimizzazione per directory con molti file)
    const html = AppState.data.paths.map((file, i) => buildPathHTML(file, i)).join("");
    UI.pathsTbody.insertAdjacentHTML("beforeend", html);
  } else {
    const note = AppState.params.q
      ? "No results"
      : AppState.data.dir_exists
        ? "Empty folder"
        : "Folder will be created when a file is uploaded";
    UI.emptyFolder.textContent = note;
    show(UI.emptyFolder);
  }
}

/**
 * Builds the HTML string for a single file/directory row (senza toccare il DOM).
 * Usato da renderPathsTableBody per il batch-insert.
 * @param {PathItem} file
 * @param {number}   index
 * @returns {string}
 */
function buildPathHTML(file, index) {
  const d = AppState.data;
  const encodedName = encodedStr(file.name);
  let url = newUrl(file.name);
  const isDir = file.path_type.endsWith("Dir");

  if (isDir) url += "/";

  // ── Action buttons ──────────────────────────────────────────
  let actionDownload = "";
  let actionMove     = "";
  let actionDelete   = "";
  let actionEdit     = "";
  let actionView     = "";
  let actionPlay     = "";

  if (isDir) {
    if (d.allow_archive) {
      actionDownload = `<a class="action-btn dlwt" href="${url}?zip" title="Download folder as .zip" download>${ICONS.download}</a>`;
    }
  } else {
    actionDownload = `<a class="action-btn dlwt" href="${url}" title="Download file" download>${ICONS.download}</a>`;
  }

  if (d.allow_delete) {
    if (d.allow_upload) {
      actionMove = `<div onclick="movePath(${index})" class="action-btn" id="moveBtn${index}" title="Move &amp; Rename">${ICONS.move}</div>`;
      if (!isDir) {
        actionEdit = `<a class="action-btn" title="Edit file" target="_blank" rel="noopener noreferrer" href="${url}?edit">${ICONS.edit}</a>`;
      }
    }
    actionDelete = `<div onclick="deletePath(${index})" class="action-btn" id="deleteBtn${index}" title="Delete">${ICONS.delete}</div>`;
  }

  if (!actionEdit && !isDir) {
    actionView = `<a class="action-btn" title="View file" target="_blank" rel="noopener noreferrer" href="${url}?view">${ICONS.view}</a>`;
  }

  const ext = file.name.toLowerCase();
  const isVideo = VIDEO_FORMATS.some(v => ext.endsWith(v));
  if (!isDir && isVideo) {
    actionPlay = `<a class="action-btn" title="Open in resilient player" target="_blank" rel="noopener noreferrer" href="${url}?view">${ICONS.play}</a>`;
  }

  // ── Public link cells ───────────────────────────────────────
  let publinkCell = "";
  let curlCell    = "";
  let wgetCell    = "";
  let mobilePublinkBtn     = "";
  let mobilePublinkPlayBtn = "";

  // An entry gets public-link cells when the server issued a token for it.
  // Folders are linkable too: their token grants the whole subtree below them.
  // The read-only public-bucket viewer is the one case without tokens, and it
  // stays file-only, exactly as before.
  const hasPublink = d.publink_enabled
    && (d.publink_is_viewer ? !isDir : typeof file.publink_token === "string");

  if (d.publink_enabled) {
    if (hasPublink) {
      const displayFileName    = file.name.split("/").pop() || file.name;
      const isPublicVideo = VIDEO_FORMATS.some(v => ext.endsWith(v));

      let publicUrl;
      if (d.publink_is_viewer) {
        // Viewer mode: file is already public, link is the direct viewer URL
        publicUrl = newUrl(file.name);
      } else if (file.publink_token) {
        // The token is an HMAC keyed with a server-side secret: it is issued by
        // the server per entry and cannot be derived in the browser.
        const currentDir   = d.href.replace(d.uri_prefix, "").replace(/^\//, "").replace(/\/$/, "");
        const relativePath = currentDir ? `${currentDir}/${file.name}` : file.name;
        const encodedRelativePath = relativePath.split("/").map(encodeURIComponent).join("/");
        publicUrl = `${window.location.origin}${d.uri_prefix}@public/${file.publink_token}/${encodedRelativePath}`;
      } else {
        publicUrl = newUrl(file.name);
      }

      const escapedPublicUrl   = escapeForOnclick(publicUrl);
      const resilientBtn  = isPublicVideo
        ? `<a href="${publicUrl}?view" class="copy-link-btn" target="_blank" rel="noopener noreferrer" title="Open in resilient player">${ICONS.play}</a>`
        : "";

      // A folder link browses, but `curl`/`wget` on it would just save a web
      // page. For folders the commands target `?zip`, so they fetch the folder
      // as an archive named after it; files keep the plain URL.
      const downloadUrl     = isDir ? `${publicUrl}?zip` : publicUrl;
      const downloadName    = isDir ? `${displayFileName}.zip` : displayFileName;
      const escapedDownloadUrl  = escapeForOnclick(downloadUrl);
      const escapedDownloadName = escapeForOnclick(downloadName);

      // Desktop: columns
      publinkCell = `
        <td class="col-publink">
          <div class="public-link-container">
            <a href="${publicUrl}" class="public-link-url" target="_blank" rel="noopener noreferrer" title="${publicUrl}">${encodedName}</a>
            <button class="copy-link-btn" onclick="copyPublicLink(this,'${escapedPublicUrl}')" title="Copy public link">
              <svg width="14" height="14" viewBox="0 0 16 16"><path d="M4 1.5H3a2 2 0 0 0-2 2V14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V3.5a2 2 0 0 0-2-2h-1v1h1a1 1 0 0 1 1 1V14a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V3.5a1 1 0 0 1 1-1h1v-1z"/><path d="M9.5 1a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5h-3a.5.5 0 0 1-.5-.5v-1a.5.5 0 0 1 .5-.5h3zm-3-1A1.5 1.5 0 0 0 5 1.5v1A1.5 1.5 0 0 0 6.5 4h3A1.5 1.5 0 0 0 11 2.5v-1A1.5 1.5 0 0 0 9.5 0h-3z"/></svg>
            </button>
            ${resilientBtn}
          </div>
        </td>`;
      curlCell = `<td class="col-publink-curl"><button class="copy-command-btn" onclick="copyCurlCommand(this,'${escapedDownloadUrl}','${escapedDownloadName}')" title="Copy curl command">Curl</button></td>`;
      wgetCell = `<td class="col-publink-wget"><button class="copy-command-btn" onclick="copyWgetCommand(this,'${escapedDownloadUrl}','${escapedDownloadName}')" title="Copy wget command">Wget</button></td>`;

      // Mobile: trigger icon in actions column. Folders pass their archive URL
      // so the sheet's curl/wget fetch the zip, not the browsable page.
      mobilePublinkBtn = `<button class="action-btn publink-mobile-btn" onclick="openPublinkSheet('${escapedPublicUrl}','${escapedDownloadName}',${isPublicVideo ? `'${escapeForOnclick(publicUrl + "?view")}'` : "null"},${isDir},'${escapedDownloadUrl}')\" title=\"Public link\">${ICONS.link}</button>`;
      if (isPublicVideo) {
        mobilePublinkPlayBtn = `<a class="action-btn mobile-publink-play" title="Open public link in resilient player" target="_blank" rel="noopener noreferrer" href="${publicUrl}?view">${ICONS.play}</a>`;
      }
    } else {
      publinkCell = `<td class="col-publink"></td>`;
      curlCell    = `<td class="col-publink-curl"></td>`;
      wgetCell    = `<td class="col-publink-wget"></td>`;
    }
  }

  const [sizeNum, sizeUnit] = isDir
    ? formatDirSizeParts(file.size)
    : formatFileSize(file.size);
  const sizeDisplay = `${sizeNum} ${sizeUnit}`;
  const sizeHtml = `<span class="size-num">${sizeNum}</span><span class="size-unit">${sizeUnit}</span>`;

  const videoClass = !isDir && isVideo ? " video-file" : "";
  const isAudio = AUDIO_FORMATS.some(v => ext.endsWith(v));
  const isMedia = !isDir && (isVideo || isAudio);
  const resumeValue = d.playback_resume_enabled && isMedia && Number.isFinite(file.resume_position)
    ? formatResumeHoursMinutes(file.resume_position)
    : "";
  const mobileResumeSuffix = d.playback_resume_enabled && isMedia
    ? `<span class="resume-mobile-inline" data-resume-inline="${index}">${resumeValue ? ` - (${resumeValue})` : ""}</span>`
    : "";
  const resumeCell = d.playback_resume_enabled
    ? `<td class="col-resume"${isMedia ? ` data-resume-cell="${index}"` : ""}>${resumeValue}</td>`
    : "";

  return `
    <tr id="addPath${index}" data-index="${index}">
      <td class="col-checkbox"><input type="checkbox" class="path-checkbox" data-index="${index}"></td>
      <td class="path col-icon">${getPathSvg(file.path_type)}</td>
      <td class="path col-name">
        <a href="${url}" class="${videoClass.trim()}" data-path="${url}" ${isDir ? "" : `target="_blank" rel="noopener noreferrer"`}>${encodedName}${mobileResumeSuffix}</a>
        <div class="row-meta">
          <span class="row-meta-size">${sizeDisplay}</span>
          <div class="col-actions-inner">
            ${actionDownload}${actionPlay}${actionView}${actionMove}${actionDelete}${actionEdit}${mobilePublinkBtn}${mobilePublinkPlayBtn}
          </div>
        </div>
      </td>
      <td class="col-mtime">${formatMtime(file.mtime)}</td>
      <td class="col-size">${sizeHtml}</td>
      ${resumeCell}
      <td class="col-actions">
        <div class="col-actions-inner">
          ${actionDownload}${actionPlay}${actionView}${actionMove}${actionDelete}${actionEdit}${mobilePublinkBtn}
        </div>
      </td>
      ${d.publink_enabled ? publinkCell + curlCell + wgetCell : ""}
    </tr>`;
}

// ═══════════════════════════════════════════════════════════════
// SECTION 9: UPLOADER CLASS
// ═══════════════════════════════════════════════════════════════

class Uploader {
  /**
   * @param {File}     file
   * @param {string[]} pathParts
   */
  constructor(file, pathParts) {
    /** @type {Element|null} */
    this.$status = null;
    this.uploaded     = 0;
    this.uploadOffset = 0;
    this.lastUptime   = 0;
    this.settled      = false;
    this.name = [...pathParts, file.name].join("/");
    this.idx  = Uploader.globalIdx++;
    this.file = file;
    this.url  = newUrl(this.name);
  }

  upload() {
    const { idx, name, url } = this;
    const encodedName = encodedStr(name);
    UI.uploadersTbody.insertAdjacentHTML("beforeend", `
      <tr id="upload${idx}" class="uploader">
        <td class="path col-icon">${getPathSvg()}</td>
        <td class="path col-name uploader-name"><a href="${url}">${encodedName}</a></td>
        <td class="col-progress" id="uploadStatus${idx}"></td>
      </tr>`);
    show(UI.uploadersTable);
    hide(UI.emptyFolder);
    this.$status = document.getElementById(`uploadStatus${idx}`);
    this.$status.innerHTML = "–";
    this.$status.addEventListener("click", e => {
      const m = /^retry(\d+)$/.exec(e.target.id);
      if (m) {
        const up = AppState.failUploaders.get(parseInt(m[1]));
        if (up) up.retry();
      }
    });
    Uploader.queues.push(this);
    Uploader.runQueue();
  }

  ajax() {
    const { url } = this;
    this.uploaded   = 0;
    this.lastUptime = Date.now();

    const xhr = new XMLHttpRequest();
    xhr.upload.addEventListener("progress", e => this.progress(e), false);
    xhr.addEventListener("readystatechange", () => {
      if (xhr.readyState === 4) {
        if (xhr.status >= 200 && xhr.status < 300) {
          this.complete();
        } else if (xhr.status !== 0) {
          this.fail(`${xhr.status} ${xhr.statusText}`);
        }
      }
    });
    xhr.addEventListener("error",  () => this.fail(), false);
    xhr.addEventListener("abort",  () => this.fail(), false);

    if (this.uploadOffset > 0) {
      xhr.open("PATCH", url);
      xhr.setRequestHeader("X-Update-Range", "append");
      xhr.send(this.file.slice(this.uploadOffset));
    } else {
      xhr.open("PUT", url);
      xhr.send(this.file);
    }
  }

  async retry() {
    const res = await fetch(this.url, { method: "HEAD" });
    let offset = 0;
    if (res.status === 200) {
      offset = parseInt(res.headers.get("content-length") || "0") || 0;
    }
    this.uploadOffset = offset;
    this.settled = false;
    Uploader.runnings++;
    this.ajax();
  }

  progress(event) {
    const now   = Date.now();
    const elapsed = now - this.lastUptime;
    if (elapsed < 300) return; // throttle update for safari
    const speed = (event.loaded - this.uploaded) / elapsed * 1000;
    const [sv, su] = formatFileSize(speed);
    const pct  = formatPercent(((event.loaded + this.uploadOffset) / this.file.size) * 100);
    const eta  = formatDuration((event.total - event.loaded) / speed);
    this.$status.innerHTML =
      `<span style="width:80px">${sv} ${su}/s</span><span>${pct} ${eta}</span>`;
    this.uploaded   = event.loaded;
    this.lastUptime = now;
  }

  complete() {
    if (this.settled) return;
    this.settled = true;
    const replacement = this.$status.cloneNode(true);
    replacement.innerHTML = `✓`;
    this.$status.parentNode.replaceChild(replacement, this.$status);
    this.$status = null;
    AppState.failUploaders.delete(this.idx);
    Uploader.runnings--;
    Uploader.runQueue();
  }

  fail(reason = "") {
    // A single transfer can raise both `error` and `abort`; without this guard
    // `runnings` goes negative and the maxUploads cap stops holding.
    if (this.settled) return;
    this.settled = true;
    this.$status.innerHTML =
      `<span style="width:20px" title="${encodedStr(reason)}">✗</span><span class="retry-btn" id="retry${this.idx}" title="Retry">↻</span>`;
    AppState.failUploaders.set(this.idx, this);
    Uploader.runnings--;
    Uploader.runQueue();
  }
}

Uploader.globalIdx = 0;
Uploader.runnings  = 0;
Uploader.authPromise = null;
/** @type {Uploader[]} */
Uploader.queues = [];

Uploader.runQueue = async () => {
  if (Uploader.runnings >= AppState.maxUploads) return;
  if (Uploader.queues.length === 0) return;
  Uploader.runnings++;
  const uploader = Uploader.queues.shift();
  // Share one in-flight check: setting a flag before awaiting let a second
  // concurrent upload start unauthenticated.
  if (!Uploader.authPromise) {
    Uploader.authPromise = checkAuth().catch(() => { Uploader.authPromise = null; });
  }
  await Uploader.authPromise;
  uploader.ajax();
};

// prevent leaving page while uploads in progress
window.addEventListener('beforeunload', (event) => {
  if (Uploader.queues.length > 0 || Uploader.runnings > 0) {
    event.preventDefault();
    event.returnValue = '';
    return '';
  }
});

// ═══════════════════════════════════════════════════════════════
// SECTION 10: UPLOAD / DROPZONE SETUP
// ═══════════════════════════════════════════════════════════════

function setupDropzone() {
  ["drag","dragstart","dragend","dragover","dragenter","dragleave","drop"].forEach(name => {
    document.addEventListener(name, e => { e.preventDefault(); e.stopPropagation(); });
  });
  document.addEventListener("drop", async e => {
    if (!e.dataTransfer.items[0].webkitGetAsEntry) {
      Array.from(e.dataTransfer.files)
        .filter(f => f.size > 0)
        .forEach(f => new Uploader(f, []).upload());
    } else {
      const entries = Array.from(e.dataTransfer.items).map(i => i.webkitGetAsEntry());
      addFileEntries(entries, []);
    }
  });
}

function setupUploadFile() {
  show(UI.btnUploadFile);
  show(UI.btnUploadFolder);
  UI.inputUploadFile.addEventListener("change", e => {
    Array.from(e.target.files).forEach(f => new Uploader(f, []).upload());
  });
  UI.inputUploadFolder.addEventListener("change", e => {
    Array.from(e.target.files).forEach(f => {
      const parts = f.webkitRelativePath ? f.webkitRelativePath.split("/").slice(0, -1) : [];
      new Uploader(f, parts).upload();
    });
  });
}

function setupNewFolder() {
  show(UI.btnNewFolder);
  UI.btnNewFolder.addEventListener("click", () => {
    const name = prompt("Enter folder name");
    if (name) createFolder(name);
  });
}

function setupNewFile() {
  show(UI.btnNewFile);
  UI.btnNewFile.addEventListener("click", () => {
    const name = prompt("Enter file name");
    if (name) createFile(name);
  });
}

function normalizeUploadLimit(value) {
  if (!Number.isFinite(value) || value < 1) return 1;
  return Math.min(value, 10);
}

function setupUploadLimitControl() {
  show(UI.uploadLimit);
  const stored  = storageGet(sessionStorage, MAX_UPLOADINGS_KEY);
  const initial = normalizeUploadLimit(stored ? parseInt(stored, 10) : AppState.maxUploads);
  AppState.maxUploads = initial;
  UI.uploadLimitInput.value   = String(initial);
  UI.uploadLimitBadge.textContent = String(initial);

  const applyValue = () => {
    const next = normalizeUploadLimit(parseInt(UI.uploadLimitInput.value, 10));
    UI.uploadLimitInput.value = String(next);
    AppState.maxUploads = next;
    storageSet(sessionStorage, MAX_UPLOADINGS_KEY, String(next));
    UI.uploadLimitBadge.textContent = String(next);
  };

  UI.uploadLimitApply.addEventListener("click", applyValue);
  UI.uploadLimitInput.addEventListener("keydown", e => {
    if (e.key === "Enter") { e.preventDefault(); applyValue(); }
  });
  UI.uploadLimitInput.addEventListener("blur", () => {
    if (!UI.uploadLimitInput.value) {
      UI.uploadLimitInput.value = String(AppState.maxUploads);
    }
  });
}

async function addFileEntries(entries, dirs) {
  for (const entry of entries) {
    if (entry.isFile) {
      entry.file(f => new Uploader(f, dirs).upload());
    } else if (entry.isDirectory) {
      const reader = entry.createReader();
      const cb = entries => {
        if (entries.length > 0) {
          addFileEntries(entries, [...dirs, entry.name]);
          reader.readEntries(cb);
        }
      };
      reader.readEntries(cb);
    }
  }
}

// ═══════════════════════════════════════════════════════════════
// SECTION 11: SEARCH
// ═══════════════════════════════════════════════════════════════

function setupSearch() {
  show(UI.searchForm);
  UI.searchForm.addEventListener("submit", e => {
    e.preventDefault();
    const q = new FormData(UI.searchForm).get("q");
    location.href = baseUrl() + (q ? "?q=" + encodeURIComponent(q) : "");
  });
  if (AppState.params.q) {
    UI.searchInput.value = AppState.params.q;
  }
}

// ═══════════════════════════════════════════════════════════════
// SECTION 12: AUTHENTICATION
// ═══════════════════════════════════════════════════════════════

async function setupAuth() {
  const d = AppState.data;
  const isPublicPage = window.location.pathname.includes("/@public/");
  if (d.user) {
    show(UI.btnLogout);
    UI.btnLogout.addEventListener("click", logout);
    UI.userName.textContent = d.user;
  } else if (!isPublicPage) {
    show(UI.btnLogin);
    UI.btnLogin.addEventListener("click", async () => {
      try { await checkAuth("login"); } catch {}
      location.reload();
    });
  }
}

async function checkAuth(variant) {
  if (!AppState.data.auth) return;
  const qs  = variant ? `?${variant}` : "";
  const res = await fetch(baseUrl() + qs, { method: "CHECKAUTH" });
  await assertResOK(res);
  hide(UI.btnLogin);
  show(UI.btnLogout);
  UI.userName.textContent = await res.text();
}

function logout() {
  if (!AppState.data.auth) return;
  const url = baseUrl();
  const xhr = new XMLHttpRequest();
  xhr.open("LOGOUT", url, true, AppState.data.user);
  xhr.onload = () => { location.href = url; };
  xhr.send();
}

function setupDownloadWithToken() {
  document.querySelectorAll("a.dlwt").forEach(link => {
    link.addEventListener("click", async e => {
      e.preventDefault();
      try {
        const el   = e.currentTarget || e.target;
        const orig = el.getAttribute("href");
        const tUrl = new URL(orig);
        tUrl.searchParams.set("tokengen", "");
        const res = await fetch(tUrl);
        if (!res.ok) throw new Error("Failed to fetch token");
        const token = await res.text();
        const dUrl  = new URL(orig);
        dUrl.searchParams.set("token", token);
        const a = document.createElement("a");
        a.href = dUrl.toString();
        a.download = "";
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
      } catch (err) {
        alert(`Failed to download, ${err.message}`);
      }
    });
  });
}

// ═══════════════════════════════════════════════════════════════
// SECTION 13: FILE OPERATIONS
// ═══════════════════════════════════════════════════════════════

/**
 * Delete a path by row index. Called via inline onclick.
 * @param {number} index
 */
async function deletePath(index) {
  const file = AppState.data.paths[index];
  if (!file) return;
  await doDeletePath(file.name, newUrl(file.name), () => {
    forgetPath(index);
    updateSelectionUI();
    if (!AppState.data.paths.find(v => !!v)) {
      hide(UI.pathsTable);
      const note = AppState.params.q ? "No results"
        : AppState.data.dir_exists ? "Empty folder"
        : "Folder will be created when a file is uploaded";
      UI.emptyFolder.textContent = note;
      show(UI.emptyFolder);
    }
  });
}

async function doDeletePath(name, url, cb) {
  if (!confirm(`Delete \`${name}\`?`)) return;
  try {
    await checkAuth();
    const res = await fetch(url, { method: "DELETE" });
    await assertResOK(res);
    cb();
  } catch (err) {
    alert(`Cannot delete \`${name}\`, ${err.message}`);
  }
}

/**
 * Move/rename a path by row index. Called via inline onclick.
 * @param {number} index
 */
async function movePath(index) {
  const file = AppState.data.paths[index];
  if (!file) return;
  const fileUrl    = newUrl(file.name);
  const newFileUrl = await doMovePath(fileUrl);
  if (newFileUrl) {
    location.href = newFileUrl.split("/").slice(0, -1).join("/");
  }
}

async function doMovePath(fileUrl) {
  const obj    = new URL(fileUrl);
  const prefix = AppState.data.uri_prefix.slice(0, -1);
  const filePath = decodeURIComponent(obj.pathname.slice(prefix.length));

  let newPath = prompt("Enter new path", filePath);
  if (!newPath) return null;
  if (!newPath.startsWith("/")) newPath = "/" + newPath;
  if (filePath === newPath) return null;
  const newFileUrl = obj.origin + prefix + newPath.split("/").map(encodeURIComponent).join("/");

  try {
    await checkAuth();
    const head = await fetch(newFileUrl, { method: "HEAD" });
    if (head.status === 200 && !confirm("Override existing file?")) return null;
    const res = await fetch(fileUrl, {
      method: "MOVE",
      headers: { "Destination": newFileUrl },
    });
    await assertResOK(res);
    return newFileUrl;
  } catch (err) {
    alert(`Cannot move \`${filePath}\` to \`${newPath}\`, ${err.message}`);
    return null;
  }
}

async function createFolder(name) {
  const url = newUrl(name);
  try {
    await checkAuth();
    const res = await fetch(url, { method: "MKCOL" });
    await assertResOK(res);
    location.href = url;
  } catch (err) {
    alert(`Cannot create folder \`${name}\`, ${err.message}`);
  }
}

async function createFile(name) {
  const url = newUrl(name);
  try {
    await checkAuth();
    const res = await fetch(url, { method: "PUT", body: "" });
    await assertResOK(res);
    location.href = url + "?edit";
  } catch (err) {
    alert(`Cannot create file \`${name}\`, ${err.message}`);
  }
}

async function saveChange() {
  try {
    await fetch(baseUrl(), { method: "PUT", body: UI.editor.value });
    location.reload();
  } catch (err) {
    alert(`Failed to save file, ${err.message}`);
  }
}

function isMediaName(name) {
  const lowerName = name.toLowerCase();
  return VIDEO_FORMATS.some(v => lowerName.endsWith(v)) || AUDIO_FORMATS.some(v => lowerName.endsWith(v));
}

function isDirectoryItem(item) {
  return !!item && item.path_type.endsWith("Dir");
}

function isMediaItem(item) {
  return !!item && !isDirectoryItem(item) && isMediaName(item.name);
}

function buildMediaUrl(pathName) {
  return newUrl(pathName.replace(/^\/+/, "")).split(/[?#]/)[0];
}

function joinPathNames(parent, child) {
  return [parent, child]
    .filter(Boolean)
    .map(part => part.replace(/^\/+|\/+$/g, ""))
    .filter(Boolean)
    .join("/");
}

function buildPlaylistEntry(pathName) {
  const normalizedPath = pathName.replace(/^\/+/, "");
  return {
    name: normalizedPath,
    url: buildMediaUrl(normalizedPath)
  };
}

function buildDirectoryListingUrl(dirName) {
  const url = new URL(newUrl(dirName).split(/[?#]/)[0]);
  if (!url.pathname.endsWith("/")) url.pathname += "/";
  url.search = "";
  url.searchParams.set("json", "");
  return url.toString();
}

async function collectPlaylistEntriesFromDirectory(dirName, playlist, seenFiles, seenDirs) {
  const normalizedDir = dirName.replace(/^\/+|\/+$/g, "");
  if (!normalizedDir || seenDirs.has(normalizedDir)) return;
  seenDirs.add(normalizedDir);

  const res = await fetch(buildDirectoryListingUrl(normalizedDir));
  await assertResOK(res);
  const data = await res.json();
  const paths = Array.isArray(data?.paths) ? data.paths : [];

  for (const item of paths) {
    if (!item || typeof item.name !== "string") continue;

    const fullName = joinPathNames(normalizedDir, item.name);
    if (isDirectoryItem(item)) {
      await collectPlaylistEntriesFromDirectory(fullName, playlist, seenFiles, seenDirs);
      continue;
    }

    if (!isMediaName(fullName) || seenFiles.has(fullName)) continue;
    seenFiles.add(fullName);
    playlist.push(fullName);
  }
}

async function collectSelectedMediaPaths(selectedItems) {
  const mediaPaths = [];
  const seenFiles = new Set();
  const seenDirs = new Set();

  for (const item of selectedItems) {
    if (isDirectoryItem(item)) {
      await collectPlaylistEntriesFromDirectory(item.name, mediaPaths, seenFiles, seenDirs);
      continue;
    }

    if (!isMediaItem(item) || seenFiles.has(item.name)) continue;
    seenFiles.add(item.name);
    mediaPaths.push(item.name);
  }

  return mediaPaths;
}

/**
 * Repaints one row's resume position, in the table cell and in the mobile
 * inline suffix. A non-finite `resumeSeconds` means "no saved position": the
 * cell is blanked, matching how a row is first rendered.
 * @param {number} index
 * @param {number} resumeSeconds
 */
function setRowResumeValue(index, resumeSeconds) {
  const file = AppState.data?.paths?.[index];
  if (!AppState.data?.playback_resume_enabled || !isMediaItem(file)) return;

  const hasPosition = Number.isFinite(resumeSeconds);
  const resumeText = hasPosition ? formatResumeHoursMinutes(resumeSeconds) : "";
  if (hasPosition) file.resume_position = resumeSeconds;
  else delete file.resume_position;

  const resumeCell = document.querySelector(`[data-resume-cell="${index}"]`);
  if (resumeCell) resumeCell.textContent = resumeText;

  const resumeInline = document.querySelector(`[data-resume-inline="${index}"]`);
  if (resumeInline) resumeInline.textContent = hasPosition ? ` - (${resumeText})` : "";
}

/**
 * Re-reads the saved playback positions from the listing endpoint and repaints
 * every media row.
 *
 * The initial page is rendered server-side, so a position saved afterwards (the
 * video was opened in another tab) is only visible after a reload. Polling
 * would keep the column fresh on its own; the caller decides when to run this.
 * @returns {Promise<boolean>} whether anything was repainted
 */
async function refreshResumePositions() {
  const d = AppState.data;
  if (!d?.playback_resume_enabled || d.kind !== "Index") return false;
  if (!Array.isArray(d.paths) || !d.paths.some(isMediaItem)) return false;

  let listing;
  try {
    const res = await fetch(buildDirectoryListingUrl(""));
    if (!res.ok) return false;
    listing = await res.json();
  } catch {
    // Offline or the folder moved: keep whatever is on screen.
    return false;
  }
  if (!Array.isArray(listing?.paths)) return false;

  // Keyed by name: the row order and the listing order are independent, and a
  // row whose file was deleted meanwhile is simply not found and left alone.
  const positions = new Map();
  for (const item of listing.paths) {
    if (isMediaItem(item)) positions.set(item.name, item.resume_position);
  }

  let repainted = false;
  d.paths.forEach((file, index) => {
    if (!isMediaItem(file) || !positions.has(file.name)) return;
    const position = positions.get(file.name);
    if (file.resume_position === position) return;
    setRowResumeValue(index, position);
    repainted = true;
  });
  return repainted;
}

/**
 * Keeps the Resume column in sync without a reload: the video player opens in a
 * new tab, so the listing sits in the background while playback progresses and
 * the position only becomes visible once the user comes back to this tab.
 *
 * Both `visibilitychange` and `focus` fire on the way back, and a hidden tab can
 * fire either again, so the refresh is rate-limited rather than debounced: a
 * returning tab updates immediately, and rapid alt-tabbing cannot re-list the
 * directory more than once per interval.
 */
function setupResumePositionSync() {
  if (!AppState.data?.playback_resume_enabled || AppState.data.kind !== "Index") return;

  // `null` until the first sync runs: a plain 0 would suppress that first run
  // whenever the clock is still near zero.
  let lastRun = null;
  const MIN_INTERVAL_MS = 30000;

  const sync = () => {
    if (document.hidden) return;
    const now = Date.now();
    if (lastRun !== null && now - lastRun < MIN_INTERVAL_MS) return;
    lastRun = now;
    void refreshResumePositions();
  };

  document.addEventListener("visibilitychange", sync);
  window.addEventListener("focus", sync);
}

// ═══════════════════════════════════════════════════════════════
// SECTION 14: MULTI-SELECTION
// ═══════════════════════════════════════════════════════════════

function setupMultiSelection() {
  const $all = document.getElementById("select-all-checkbox");
  if ($all) $all.addEventListener("change", onSelectAll);

  document.querySelectorAll(".path-checkbox").forEach($cb => {
    $cb.addEventListener("change", onCheckboxChange);
  });

  UI.selBtnDownload.addEventListener("click", e => { e.preventDefault(); downloadSelectedAsZip(); });
  UI.selBtnMove.addEventListener("click",     e => { e.preventDefault(); moveSelected(); });
  UI.selBtnDelete.addEventListener("click",   e => { e.preventDefault(); deleteSelected(); });
  UI.selBtnPlaylist.addEventListener("click", e => { e.preventDefault(); startPlaylist(); });
  UI.selBtnResumeReset.addEventListener("click", e => { e.preventDefault(); resetSelectedResume(); });
  UI.selBtnClear.addEventListener("click",    e => { e.preventDefault(); clearSelection(); });
}

function onSelectAll(e) {
  const checked = e.target.checked;
  document.querySelectorAll(".path-checkbox").forEach($cb => {
    $cb.checked = checked;
    const idx = parseInt($cb.dataset.index);
    const row = document.querySelector(`tr[data-index="${idx}"]`);
    if (checked) { AppState.selection.add(idx); row?.classList.add("selected"); }
    else         { AppState.selection.delete(idx); row?.classList.remove("selected"); }
  });
  updateSelectionUI();
}

function onCheckboxChange(e) {
  const idx = parseInt(e.target.dataset.index);
  const row = document.querySelector(`tr[data-index="${idx}"]`);
  if (e.target.checked) { AppState.selection.add(idx); row?.classList.add("selected"); }
  else                  { AppState.selection.delete(idx); row?.classList.remove("selected"); }

  const $all   = document.getElementById("select-all-checkbox");
  const total  = document.querySelectorAll(".path-checkbox").length;
  if ($all) $all.checked = AppState.selection.size === total;

  updateSelectionUI();
}

function updateSelectionUI() {
  const count = AppState.selection.size;
  const d     = AppState.data;

  if (count > 0) {
    show(UI.selectionToolbar);
    UI.selectionCount.textContent = `${count} selected`;

    if (d.allow_archive)                   show(UI.selBtnDownload);
    if (d.allow_delete && d.allow_upload)  show(UI.selBtnMove);
    if (d.allow_delete)                    show(UI.selBtnDelete);

    const hasPlaylistCandidates = Array.from(AppState.selection).some(i => {
      const f = d.paths[i];
      return isDirectoryItem(f) || isMediaItem(f);
    });
    if (hasPlaylistCandidates) show(UI.selBtnPlaylist);
    else hide(UI.selBtnPlaylist);

    if (d.playback_resume_enabled && hasPlaylistCandidates) show(UI.selBtnResumeReset);
    else hide(UI.selBtnResumeReset);
  } else {
    hide(UI.selectionToolbar);
    AppState.selection.clear();
  }
}

/**
 * The current selection as `{ index, file }` pairs, skipping rows that have
 * already been removed. Callers need the index, so it must travel with the
 * entry rather than be recovered by position afterwards.
 * @returns {{index: number, file: PathItem}[]}
 */
function selectedEntries() {
  const d = AppState.data;
  return Array.from(AppState.selection)
    .map(index => ({ index, file: d.paths[index] }))
    .filter(entry => !!entry.file);
}

/**
 * Drop a path from the model, the table and the selection. Leaving a removed
 * index in the selection is what desynchronised the bulk operations.
 * @param {number} index
 */
function forgetPath(index) {
  AppState.data.paths[index] = null;
  document.getElementById(`addPath${index}`)?.remove();
  AppState.selection.delete(index);
}

function clearSelection() {
  AppState.selection.clear();
  document.querySelectorAll(".path-checkbox").forEach($cb => { $cb.checked = false; });
  document.querySelectorAll("tr.selected").forEach(row => row.classList.remove("selected"));
  const $all = document.getElementById("select-all-checkbox");
  if ($all) $all.checked = false;
  updateSelectionUI();
}

async function downloadSelectedAsZip() {
  if (AppState.selection.size === 0) return;
  const d = AppState.data;
  const files = Array.from(AppState.selection)
    .map(i => d.paths[i])
    .filter(Boolean);
  if (!files.length) { alert("No valid files selected"); return; }

  // Single directory → native ZIP
  if (files.length === 1 && files[0].path_type.endsWith("Dir")) {
    const file = files[0];
    let url    = newUrl(file.name) + "/?zip";
    if (d.user) {
      try {
        await checkAuth();
        const tUrl = new URL(url);
        tUrl.searchParams.set("tokengen", "");
        const res = await fetch(tUrl);
        if (!res.ok) throw new Error("Failed to fetch token");
        const token = await res.text();
        const dUrl  = new URL(url);
        dUrl.searchParams.set("token", token);
        url = dUrl.toString();
      } catch (err) {
        alert(`Failed to download: ${err.message}`);
        return;
      }
    }
    const a = document.createElement("a");
    a.href = url; a.download = file.name + ".zip";
    document.body.appendChild(a); a.click(); document.body.removeChild(a);
    return;
  }

  const names = Array.from(new Set(files.map(f => f.name)));
  if (!names.length)                             { alert("No valid files selected"); return; }
  if (names.length > MAX_MULTI_ZIP_SELECTION)    { alert(`Too many items. Max ${MAX_MULTI_ZIP_SELECTION}.`); return; }
  const selectionJson = JSON.stringify(names);
  if (selectionJson.length > MAX_MULTI_ZIP_PAYLOAD) { alert("Selection too large."); return; }

  const base = new URL(baseUrl());
  base.searchParams.set("zip", "multi");
  base.searchParams.set("paths", selectionJson);

  try {
    let downloadUrl = new URL(base.toString());
    if (d.user) {
      await checkAuth();
      const tUrl = new URL(base.toString());
      tUrl.searchParams.set("tokengen", "");
      const res = await fetch(tUrl);
      if (!res.ok) throw new Error("Failed to fetch token");
      downloadUrl = new URL(base.toString());
      downloadUrl.searchParams.set("token", await res.text());
    }
    const head = await fetch(downloadUrl.toString(), { method: "HEAD" });
    if (!head.ok) throw new Error((await head.text()) || `Error ${head.status}`);
    const a = document.createElement("a");
    a.href = downloadUrl.toString(); a.download = "selection.zip";
    document.body.appendChild(a); a.click(); document.body.removeChild(a);
  } catch (err) {
    alert(`Failed to download selection: ${err.message}`);
  }
}

async function moveSelected() {
  if (AppState.selection.size === 0) return;
  const d     = AppState.data;
  // Keep each row's index attached to its entry. Deriving it later from
  // `selection[files.indexOf(file)]` breaks as soon as one selected index has
  // been nulled (the per-row delete button does exactly that): the filtered
  // and unfiltered arrays no longer line up and the wrong row is marked gone.
  const entries = selectedEntries();
  if (!entries.length) { alert("No valid files selected"); return; }
  const files = entries.map(e => e.file);

  const currentPath = d.href.replace(d.uri_prefix, "").replace(/^\//, "").replace(/\/$/, "");
  const defaultDest = currentPath ? `/${currentPath}/` : "/";
  let destPath = prompt(`Move ${files.length} item(s) to:`, defaultDest);
  if (!destPath) return;
  if (!destPath.startsWith("/")) destPath = "/" + destPath;
  if (!destPath.endsWith("/"))   destPath += "/";

  try {
    await checkAuth();
    let successCount = 0;
    const failedFiles = [];

    for (const { index: idx, file } of entries) {
      const fileUrl = newUrl(file.name);
      const obj     = new URL(fileUrl);
      const prefix  = d.uri_prefix.slice(0, -1);
      const newFilePath = destPath + file.name;
      const newFileUrl  = obj.origin + prefix + newFilePath.split("/").map(encodeURIComponent).join("/");

      try {
        const head = await fetch(newFileUrl, { method: "HEAD" });
        if (head.status === 200) {
          if (!confirm(`File "${file.name}" already exists. Override?`)) {
            failedFiles.push(`${file.name} (skipped)`);
            continue;
          }
        }
        const res = await fetch(fileUrl, {
          method: "MOVE",
          headers: { "Destination": newFileUrl },
        });
        if (res.ok) {
          successCount++;
          forgetPath(idx);
        } else {
          throw new Error((await res.text()) || `Status ${res.status}`);
        }
      } catch (err) {
        failedFiles.push(`${file.name} (${err.message})`);
      }
    }

    let msg = `Moved ${successCount} of ${files.length} item(s)`;
    if (failedFiles.length) msg += `\n\nFailed:\n${failedFiles.join("\n")}`;
    alert(msg);
    clearSelection();
    if (!d.paths.find(v => !!v)) {
      hide(UI.pathsTable);
      UI.emptyFolder.textContent = AppState.params.q ? "No results" : "Empty folder";
      show(UI.emptyFolder);
    }
  } catch (err) {
    alert(`Failed to move files: ${err.message}`);
  }
}

async function deleteSelected() {
  if (AppState.selection.size === 0) return;
  const d     = AppState.data;
  const entries = selectedEntries();
  if (!entries.length) { alert("No valid files selected"); return; }
  const files = entries.map(e => e.file);

  if (!confirm(`Delete ${files.length} item(s)?\n\n${files.map(f => f.name).join(", ")}`)) return;

  try {
    await checkAuth();
    let successCount = 0;
    const failedFiles = [];

    for (const { index: idx, file } of entries) {
      try {
        const res = await fetch(newUrl(file.name), { method: "DELETE" });
        if (res.ok) {
          successCount++;
          forgetPath(idx);
        } else {
          throw new Error((await res.text()) || `Status ${res.status}`);
        }
      } catch (err) {
        failedFiles.push(`${file.name} (${err.message})`);
      }
    }

    let msg = `Deleted ${successCount} of ${files.length} item(s)`;
    if (failedFiles.length) msg += `\n\nFailed:\n${failedFiles.join("\n")}`;
    alert(msg);
    clearSelection();
    if (!d.paths.find(v => !!v)) {
      hide(UI.pathsTable);
      UI.emptyFolder.textContent = AppState.params.q ? "No results" : "Empty folder";
      show(UI.emptyFolder);
    }
  } catch (err) {
    alert(`Failed to delete files: ${err.message}`);
  }
}

// ═══════════════════════════════════════════════════════════════
// SECTION 15: DISK STATISTICS
// ═══════════════════════════════════════════════════════════════

async function loadDiskStatsAsync() {
  try {
    const res = await fetch("?disk-stats");
    if (!res.ok) return;
    const data = await res.json();
    if (data.disk_usage !== undefined) {
      updateDiskStats(data.disk_usage, data.disk_total, data.disk_available);
    }
  } catch {}
}

function updateDiskStats(used, _total, available) {
  if (!UI.diskStats || !UI.diskStatsValue) return;

  if (!used) {
    UI.diskStatsValue.textContent = "Non disponibile";
    show(UI.diskStats);
    return;
  }

  const [usedSize, usedUnit] = formatFileSize(used);
  // >100 TB is typically S3 / network mount — treat as unavailable
  if (!available || available <= 0 || available > 109951162777600) {
    UI.diskStatsValue.textContent = `${usedSize} ${usedUnit} used / Non disponibile`;
    show(UI.diskStats);
    return;
  }

  const [availSize, availUnit] = formatFileSize(available);
  UI.diskStatsValue.textContent = `${usedSize} ${usedUnit} used / ${availSize} ${availUnit} available`;
  show(UI.diskStats);
}

// ═══════════════════════════════════════════════════════════════
// SECTION 16: PUBLIC LINKS
// ═══════════════════════════════════════════════════════════════

/**
 * Generate a deterministic public token from a file path.
 * @param {string} fullPath
 * @returns {string}
 */
/** Called from inline onclick in table rows. */
function copyPublicLink(btn, url) {
  copyTextToClipboard(btn, url);
}

function copyCurlCommand(btn, url, fileName) {
  copyTextToClipboard(btn, `curl -o "${fileName}" -L "${url}"`);
}

/**
 * @param {string} url
 * @param {string} [fileName] explicit output name. A folder link is archived
 *   (`?zip`), so naming the file keeps the command predictable instead of
 *   relying on the server's Content-Disposition.
 */
function copyWgetCommand(btn, url, fileName) {
  copyTextToClipboard(btn, fileName ? `wget -O "${fileName}" "${url}"` : `wget "${url}"`);
}

// ── Mobile bottom sheet ──────────────────────────────────────

/** @type {{ url: string, fileName: string, viewUrl: string|null }|null} */
let _sheetData = null;

function setupPublinkSheet() {
  if (!AppState.data.publink_enabled) return;

  UI.publinkBackdrop.addEventListener("click", closePublinkSheet);
  // "Copy" shares the link itself; only curl/wget fetch the archive, because a
  // folder link browses rather than downloads.
  UI.publinkSheetCopy.addEventListener("click", () => {
    if (_sheetData) copyTextToClipboard(UI.publinkSheetCopy, _sheetData.url);
  });
  UI.publinkSheetCurl.addEventListener("click", () => {
    if (_sheetData) {
      copyTextToClipboard(UI.publinkSheetCurl,
        `curl -o "${_sheetData.downloadName}" -L "${_sheetData.downloadUrl}"`);
    }
  });
  UI.publinkSheetWget.addEventListener("click", () => {
    if (_sheetData) {
      copyTextToClipboard(UI.publinkSheetWget,
        `wget -O "${_sheetData.downloadName}" "${_sheetData.downloadUrl}"`);
    }
  });
}

/**
 * Open the mobile public-link bottom sheet.
 * Called from inline onclick on the mobile link icon.
 * @param {string}      url        browsable link, shown in the sheet
 * @param {string}      fileName
 * @param {string|null} viewUrl
 * @param {boolean}     [isDir]     a folder: the commands fetch `?zip` instead
 * @param {string}      [zipUrl]    the archive URL, when `isDir`
 */
function openPublinkSheet(url, fileName, viewUrl, isDir, zipUrl) {
  // A folder browses but is archived for download: `curl`/`wget` on the plain
  // link would only save an HTML page.
  const downloadUrl  = isDir ? (zipUrl || `${url}?zip`) : url;
  const downloadName = isDir ? `${fileName}.zip` : fileName;
  _sheetData = { url, fileName, viewUrl, downloadUrl, downloadName };
  UI.publinkSheetUrl.href        = url;
  UI.publinkSheetUrl.textContent = url;
  if (viewUrl) {
    UI.publinkSheetPlay.href = viewUrl;
    show(UI.publinkSheetPlay);
  } else {
    hide(UI.publinkSheetPlay);
  }
  show(UI.publinkBackdrop);
  show(UI.publinkSheet);
  requestAnimationFrame(() => {
    UI.publinkBackdrop.classList.add("sheet-open");
    UI.publinkSheet.classList.add("sheet-open");
  });
}

function closePublinkSheet() {
  UI.publinkBackdrop.classList.remove("sheet-open");
  UI.publinkSheet.classList.remove("sheet-open");
  const onEnd = () => {
    hide(UI.publinkBackdrop);
    hide(UI.publinkSheet);
    UI.publinkBackdrop.removeEventListener("transitionend", onEnd);
  };
  UI.publinkBackdrop.addEventListener("transitionend", onEnd);
}

// ═══════════════════════════════════════════════════════════════
// SECTION 17: RESILIENT VIDEO PLAYER
// ═══════════════════════════════════════════════════════════════

function detectBlackVideoBorders(pixels, width, height) {
  const darkLineRatio = 0.985;
  const darkPixelLimit = 24;
  const meanLumaLimit = 12;

  const isDarkLine = (offset, stride, length) => {
    let darkPixels = 0;
    let lumaTotal = 0;
    for (let i = 0, pos = offset; i < length; i++, pos += stride) {
      const luma = (54 * pixels[pos] + 183 * pixels[pos + 1] +
        19 * pixels[pos + 2]) >> 8;
      lumaTotal += luma;
      if (luma <= darkPixelLimit) darkPixels++;
    }
    return darkPixels / length >= darkLineRatio &&
      lumaTotal / length <= meanLumaLimit;
  };

  let top = 0;
  while (top < height && isDarkLine(top * width * 4, 4, width)) top++;
  let bottom = 0;
  while (bottom < height - top &&
    isDarkLine((height - 1 - bottom) * width * 4, 4, width)) bottom++;
  let left = 0;
  while (left < width && isDarkLine(left * 4, width * 4, height)) left++;
  let right = 0;
  while (right < width - left &&
    isDarkLine((width - 1 - right) * 4, width * 4, height)) right++;

  // Only accept paired, approximately symmetric borders. This avoids treating
  // dark scenes, fades or a deliberately black edge as letterboxing.
  const symmetricPair = (first, second, size) => {
    const minimum = Math.max(2, Math.floor(size * 0.015));
    const tolerance = Math.max(2, Math.floor(size * 0.025));
    return first >= minimum && second >= minimum &&
      Math.abs(first - second) <= tolerance ? [first, second] : [0, 0];
  };

  [top, bottom] = symmetricPair(top, bottom, height);
  [left, right] = symmetricPair(left, right, width);

  // Never discard a candidate large enough to risk hiding real content.
  if (width - left - right < width * 0.55 ||
      height - top - bottom < height * 0.55) {
    return { top: 0, right: 0, bottom: 0, left: 0 };
  }
  return { top, right, bottom, left };
}

function setupAutoLetterboxFit(video) {
  if (!video || video.dataset.autoLetterboxFit === "true") return;
  video.dataset.autoLetterboxFit = "true";

  if (!window.CSS || !CSS.supports("object-view-box", "inset(1%)")) return;

  const canvas = document.createElement("canvas");
  const ctx = canvas.getContext("2d", { willReadFrequently: true });
  if (!ctx) return;

  let sourceKey = "";
  let previousCandidate = null;
  let confirmations = 0;
  let appliedViewBox = "none";
  let lastSampleAt = 0;
  let disabled = false;

  const resetForSource = () => {
    previousCandidate = null;
    confirmations = 0;
    appliedViewBox = "none";
    lastSampleAt = 0;
    video.style.removeProperty("--video-view-box");
  };

  const sameCandidate = (a, b) => a && b &&
    ["top", "right", "bottom", "left"].every(edge =>
      Math.abs(a[edge] - b[edge]) <= 0.008);

  const sampleFrame = () => {
    if (disabled || video.readyState < 2 || !video.videoWidth || !video.videoHeight) return;

    const now = performance.now();
    const sampleInterval = appliedViewBox === "none" ? 900 : 8000;
    if (now - lastSampleAt < sampleInterval) return;
    lastSampleAt = now;

    const scale = Math.min(1, 480 / video.videoWidth, 270 / video.videoHeight);
    const width = Math.max(1, Math.round(video.videoWidth * scale));
    const height = Math.max(1, Math.round(video.videoHeight * scale));
    canvas.width = width;
    canvas.height = height;

    try {
      ctx.drawImage(video, 0, 0, width, height);
      const pixels = ctx.getImageData(0, 0, width, height).data;
      const borders = detectBlackVideoBorders(pixels, width, height);
      const candidate = {
        top: borders.top / height,
        right: borders.right / width,
        bottom: borders.bottom / height,
        left: borders.left / width,
      };

      if (sameCandidate(candidate, previousCandidate)) confirmations++;
      else {
        previousCandidate = candidate;
        confirmations = 1;
      }
      if (confirmations < 3) return;

      const hasBorders = Object.values(candidate).some(value => value > 0);
      const viewBox = hasBorders
        ? `inset(${(candidate.top * 100).toFixed(3)}% ` +
          `${(candidate.right * 100).toFixed(3)}% ` +
          `${(candidate.bottom * 100).toFixed(3)}% ` +
          `${(candidate.left * 100).toFixed(3)}%)`
        : "none";
      if (viewBox === appliedViewBox) return;

      appliedViewBox = viewBox;
      video.style.setProperty("--video-view-box", viewBox);
    } catch {
      // A cross-origin or protected stream cannot be sampled safely. In that
      // case retain the normal, uncropped `contain` behaviour.
      disabled = true;
      resetForSource();
    }
  };

  const scheduleSample = () => requestAnimationFrame(sampleFrame);
  video.addEventListener("loadedmetadata", () => {
    const nextSource = video.currentSrc.split(/[?#]/)[0];
    if (nextSource !== sourceKey) {
      sourceKey = nextSource;
      disabled = false;
      resetForSource();
    }
  });
  video.addEventListener("loadeddata", scheduleSample);
  video.addEventListener("playing", scheduleSample);
  video.addEventListener("timeupdate", scheduleSample);
  video.addEventListener("seeked", scheduleSample);
}

function getFullscreenElement() {
  return document.fullscreenElement || document.webkitFullscreenElement || null;
}

function setupCustomVideoFullscreen({ wrapper, button, toolbar, panel = null, backdrop = null }) {
  // `toolbar` may be a single element (playlist) or a list of toolbars: the
  // resilient player stacks a boost bar and a seek bar, both auto-hidden.
  const toolbars = toolbar == null ? [] :
    (toolbar.length === undefined ? [toolbar] : Array.from(toolbar));
  if (!wrapper || !button || !toolbars.length) return;

  const canRequest = typeof wrapper.requestFullscreen === "function" ||
    typeof wrapper.webkitRequestFullscreen === "function";
  if (!canRequest) {
    button.hidden = true;
    return;
  }

  let hideTimer = null;
  let orientationLockRequested = false;

  const isActive = () => getFullscreenElement() === wrapper &&
    wrapper.classList.contains("custom-fit-fullscreen");

  const setControlsVisible = visible => {
    toolbars.forEach(bar => bar.classList.toggle("toolbar-hidden", !visible));
    wrapper.classList.toggle("fullscreen-controls-hidden", !visible);
    if (panel) panel.classList.toggle("panel-hidden", !visible);
    if (backdrop) backdrop.classList.toggle("fullscreen-hidden", !visible);
  };

  const clearAutoHide = () => {
    clearTimeout(hideTimer);
    hideTimer = null;
  };

  const scheduleAutoHide = () => {
    clearAutoHide();
    if (!isActive()) return;
    hideTimer = setTimeout(() => setControlsVisible(false), 3000);
  };

  const revealControls = () => {
    if (!isActive()) return;
    setControlsVisible(true);
    scheduleAutoHide();
  };

  const updateButton = active => {
    const label = active ? "Exit fitted fullscreen" : "Enter fitted fullscreen";
    button.innerHTML = active ? ICONS.exitFullscreen : ICONS.enterFullscreen;
    button.title = label;
    button.setAttribute("aria-label", label);
    button.setAttribute("aria-pressed", String(active));
  };

  const lockLandscapeOrientation = async () => {
    const orientation = window.screen?.orientation;
    if (orientationLockRequested || !orientation || typeof orientation.lock !== "function") return;

    orientationLockRequested = true;
    try {
      await orientation.lock("landscape");
    } catch {
      orientationLockRequested = false;
    }
  };

  const unlockOrientation = () => {
    if (!orientationLockRequested) return;
    orientationLockRequested = false;
    try {
      window.screen.orientation.unlock();
    } catch {
      // The fullscreen exit must remain functional when unlock is unsupported.
    }
  };

  const handleFullscreenChange = () => {
    const active = isActive();
    if (!active) {
      wrapper.classList.remove("custom-fit-fullscreen");
      unlockOrientation();
    }
    updateButton(active);
    clearAutoHide();
    setControlsVisible(true);
    if (active) {
      void lockLandscapeOrientation();
      scheduleAutoHide();
    }
  };

  const requestCustomFullscreen = async () => {
    wrapper.classList.add("custom-fit-fullscreen");
    try {
      if (wrapper.requestFullscreen) await wrapper.requestFullscreen();
      else wrapper.webkitRequestFullscreen();
      if (getFullscreenElement() === wrapper) await lockLandscapeOrientation();
    } catch {
      wrapper.classList.remove("custom-fit-fullscreen");
      unlockOrientation();
      handleFullscreenChange();
    }
  };

  const exitCustomFullscreen = async () => {
    if (document.exitFullscreen) await document.exitFullscreen();
    else if (document.webkitExitFullscreen) document.webkitExitFullscreen();
  };

  button.addEventListener("click", async () => {
    try {
      if (getFullscreenElement() === wrapper) await exitCustomFullscreen();
      else await requestCustomFullscreen();
    } catch {
      wrapper.classList.remove("custom-fit-fullscreen");
      handleFullscreenChange();
    } finally {
      button.blur();
    }
  });

  wrapper.addEventListener("pointermove", revealControls);
  wrapper.addEventListener("pointerdown", revealControls);
  toolbars.forEach(bar => {
    bar.addEventListener("pointerenter", () => {
      if (isActive()) {
        setControlsVisible(true);
        clearAutoHide();
      }
    });
    bar.addEventListener("pointerleave", scheduleAutoHide);
  });
  document.addEventListener("keydown", revealControls);
  document.addEventListener("fullscreenchange", handleFullscreenChange);
  document.addEventListener("webkitfullscreenchange", handleFullscreenChange);
  document.addEventListener("fullscreenerror", handleFullscreenChange);
  document.addEventListener("webkitfullscreenerror", handleFullscreenChange);
  updateButton(false);
}

const SEEK_STEP_SECONDS = 30;

/**
 * Target position for a relative seek, clamped to the playable range.
 * @param {number} currentTime
 * @param {number} delta         seconds, negative to rewind
 * @param {number} duration      total length in seconds
 * @returns {number|null} the new position, or null when the media cannot be
 *   seeked yet (unknown duration, live stream, metadata not loaded)
 */
function clampSeekTime(currentTime, delta, duration) {
  if (!Number.isFinite(duration) || duration <= 0) return null;
  const from = Number.isFinite(currentTime) ? Math.max(0, currentTime) : 0;
  // Stay a hair before the end: seeking exactly to `duration` fires `ended`
  // and stops playback, which is not what a "skip forward" is expected to do.
  return Math.min(Math.max(0, from + delta), Math.max(0, duration - 0.25));
}

/**
 * Wires the resilient player's seek bar: two buttons moving playback by
 * SEEK_STEP_SECONDS in either direction, clamped to the media bounds.
 * @param {HTMLVideoElement} video
 * @param {ParentNode} scope  element containing the .seek-btn buttons
 */
function setupSeekControls(video, scope) {
  const backBtn = scope.querySelector("#resilient-seek-back");
  const forwardBtn = scope.querySelector("#resilient-seek-forward");
  if (!backBtn || !forwardBtn) return;

  const seekBy = delta => {
    const target = clampSeekTime(video.currentTime, delta, video.duration);
    if (target === null) return;
    video.currentTime = target;
  };

  backBtn.addEventListener("click", () => {
    seekBy(-SEEK_STEP_SECONDS);
    backBtn.blur();
  });
  forwardBtn.addEventListener("click", () => {
    seekBy(SEEK_STEP_SECONDS);
    forwardBtn.blur();
  });
}

function setupResilientVideoPlayer() {
  const video = document.getElementById("resilientVideo");
  if (!video) return;
  const wrapper = video.closest(".video-wrapper");
  setupAutoLetterboxFit(video);
  setupCustomVideoFullscreen({
    wrapper,
    button: document.getElementById("resilient-fullscreen"),
    toolbar: wrapper.querySelectorAll(".video-toolbar"),
  });
  setupSeekControls(video, wrapper);

  // Reconnection budget — server-supplied (CLI/env/YAML), with safe fallbacks.
  // maxRetry: -1 = unlimited, 0 = disabled, N>0 = capped.
  const maxRetry = Number.isInteger(AppState.data.resilient_player_max_retry)
    ? AppState.data.resilient_player_max_retry : 30;
  const reconnectIntervalMs = Math.max(1,
    AppState.data.resilient_player_reconnect_interval ?? 10) * 1000;

  let retryCount      = 0;
  let retryTimeout    = null;
  let stalledTimeout  = null;  // A: stalled guard
  let bufferWatchdog  = null;  // E: buffer watchdog
  let savedPosition   = 0;
  let wasPlaying      = false;
  let isRecovering    = false;
  // Preserves the user's play intent across recovery cycles.
  // wasPlaying gets corrupted by pause events emitted when video.src changes;
  // autoplayIntent is set once when recovery starts and cleared only on real success.
  let autoplayIntent  = false;
  // UA-initiated pause while tab hidden (screen-off) — must not erase play intent
  let autoPaused      = false;
  // Cleanup hook for the in-flight recovery attempt's listeners/timers
  let recoveryCleanup = null;
  const baseVideoUrl  = video.src.split("?")[0];

  // E: last buffered-ahead snapshot for watchdog comparison
  let lastBufferedEnd = 0;
  let lastBufferedCheck = 0;

  let videoPath = baseVideoUrl.startsWith("/")
    ? baseVideoUrl
    : new URL(baseVideoUrl).pathname;

  // Public link detection: save the @public URL *before* extracting the private path,
  // so resume API calls can be routed through the public handler (no auth required).
  const isPublicLink  = videoPath.startsWith("/@public/");
  const resumeApiPath = videoPath; // /@public/token/path  OR  /private/path
  if (isPublicLink) {
    const parts = videoPath.split("/");
    if (parts.length >= 4) videoPath = "/" + parts.slice(3).join("/");
  }

  video.preload = "auto";

  // ── Audio boost (Web Audio API) ──────────────────────────────
  const BOOST_KEY   = "sdufs_boost:" + videoPath;
  let audioCtx      = null;
  let gainNode      = null;
  const boostBtns   = video.closest(".video-wrapper").querySelectorAll(".boost-btn");
  let savedBoost    = parseFloat(storageGet(localStorage, BOOST_KEY) || "1.0") || 1.0;

  // Restore visual active state immediately
  boostBtns.forEach(btn => {
    btn.classList.toggle("active", parseFloat(btn.dataset.gain) === savedBoost);
  });


  function ensureAudioChain() {
    if (audioCtx) { if (audioCtx.state === "suspended") audioCtx.resume(); return; }
    audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    const src  = audioCtx.createMediaElementSource(video);
    gainNode   = audioCtx.createGain();
    const comp = audioCtx.createDynamicsCompressor();
    comp.threshold.value = -6; comp.knee.value = 3; comp.ratio.value = 4;
    comp.attack.value = 0.005; comp.release.value = 0.1;
    src.connect(gainNode); gainNode.connect(comp); comp.connect(audioCtx.destination);
  }

  function setBoost(gain) {
    ensureAudioChain();
    gainNode.gain.value = gain;
    storageSet(localStorage, BOOST_KEY, String(gain));
  }

  boostBtns.forEach(btn => {
    btn.addEventListener("click", () => {
      boostBtns.forEach(b => b.classList.remove("active"));
      btn.classList.add("active");
      setBoost(parseFloat(btn.dataset.gain));
    });
  });

  // Apply saved boost on first play (AudioContext needs user gesture)
  if (savedBoost !== 1.0) {
    video.addEventListener("play", () => setBoost(savedBoost), { once: true });
  }

  // Load resume position (routed via resumeApiPath — works for both private and public links)
  loadResumePosition(resumeApiPath).then(resumeData => {
    if (resumeData && resumeData.position > 0) {
      const resumePos = Math.max(0, resumeData.position - 10);
      if (savedPosition === 0) savedPosition = resumePos;
      const onCanPlay = () => {
        video.removeEventListener("canplay", onCanPlay);
        // If a recovery cycle started meanwhile, it owns the seek position
        if (isRecovering) return;
        video.currentTime = resumePos;
        showVideoMessage(`Resumed from ${formatTime(resumePos)}`);
        setTimeout(hideVideoMessage, 2000);
      };
      video.addEventListener("canplay", onCanPlay, { once: true });
    }
  }).catch(() => {});

  // Auto-save every 5 seconds
  const saveInterval = setInterval(() => {
    if (!isRecovering && video.currentTime > 0 && !video.ended) {
      saveResumePosition(resumeApiPath, video.currentTime, video.duration);
    }
  }, 5000);

  video.addEventListener("timeupdate", () => {
    if (!isRecovering && video.currentTime > 0) savedPosition = video.currentTime;
  });

  video.addEventListener("play",  () => { wasPlaying = true; autoPaused = false; });
  video.addEventListener("pause", () => {
    autoPaused = document.hidden && wasPlaying;
    wasPlaying = false;
    clearStalledTimeout(); // A: cancel any pending stalled recovery
    if (video.currentTime > 0 && !video.ended) {
      saveResumePosition(resumeApiPath, video.currentTime, video.duration);
    }
  });
  video.addEventListener("waiting", () => {
    if (!isRecovering && wasPlaying && !video.ended) autoplayIntent = true;
  });
  // F: reset retryCount only when playback is actually running (not just on play call)
  video.addEventListener("playing", () => {
    wasPlaying = true;
    isRecovering = false;
    autoplayIntent = false; // clear intent — playback confirmed running
    autoPaused = false;
    retryCount = 0;
    clearStalledTimeout(); // A: playback resumed — cancel any pending stalled guard
    hideVideoMessage();
  });
  video.addEventListener("ended", () => { clearResumePosition(resumeApiPath); });

  // Filter spurious / fatal error events. Safari fires `error` on src reassignment,
  // user/UA aborts, and decode hiccups; treating all of them as "lost connection"
  // causes an infinite reconnect loop. Only MEDIA_ERR_NETWORK warrants recovery.
  video.addEventListener("error", () => {
    if (isRecovering) return;
    const err = video.error;
    if (!err || err.code === MediaError.MEDIA_ERR_ABORTED) return;
    if (err.code === MediaError.MEDIA_ERR_DECODE ||
        err.code === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED) {
      showVideoMessage("Playback error: media cannot be decoded.");
      return;
    }
    triggerRecovery();
  });

  // A: stalled guard — wait 15s before triggering recovery to avoid false positives
  // on slow connections that are still buffering normally.
  video.addEventListener("stalled", () => {
    if (!isRecovering && wasPlaying && !stalledTimeout) {
      autoplayIntent = true;
      stalledTimeout = setTimeout(() => {
        stalledTimeout = null;
        if (!isRecovering && wasPlaying) triggerRecovery();
      }, 15000);
    }
  });

  window.addEventListener("online", () => {
    if (autoplayIntent || wasPlaying || isRecovering) {
      const forceRecovery = autoplayIntent || isRecovering;
      if (!forceRecovery && video.readyState >= 3 && !video.paused && !video.ended) return;
      const shouldAutoplay = autoplayIntent || wasPlaying;
      autoplayIntent = shouldAutoplay;
      retryCount = 0;
      // Force-restart even if a recovery cycle is already in progress:
      // the previous attempts all failed due to no connectivity.
      clearTimeout(retryTimeout);
      cancelRecoveryAttempt();
      isRecovering = false;
      triggerRecovery({ force: forceRecovery, immediate: forceRecovery });
    }
  });

  // D: recover when tab becomes visible again after being hidden (screen-off, tab switch).
  // autoPaused covers UA-initiated pauses that already cleared wasPlaying.
  document.addEventListener("visibilitychange", () => {
    if (document.hidden || isRecovering) return;
    if (!(wasPlaying || autoPaused)) return;
    autoPaused = false;
    if (video.readyState >= 3 && video.paused && !video.ended) {
      // Stream still healthy — a plain play() is enough
      video.play().catch(() => {
        autoplayIntent = true;
        retryCount = 0;
        triggerRecovery();
      });
    } else if (video.paused || video.readyState < 3) {
      autoplayIntent = true;
      retryCount = 0;
      triggerRecovery();
    }
  });

  // E: buffer watchdog — polls every 10s while playing; if buffered-ahead hasn't grown
  // and we're within 5s of the buffer edge, the stream is silently stuck → recover.
  bufferWatchdog = setInterval(() => {
    if (!wasPlaying || isRecovering || video.paused || video.ended) return;
    const buf = video.buffered;
    if (!buf || buf.length === 0) return;
    const bufferedEnd = buf.end(buf.length - 1);
    const ahead = bufferedEnd - video.currentTime;
    const now = Date.now();
    // If buffer has grown since last check, all good — update snapshot and continue.
    if (bufferedEnd > lastBufferedEnd + 0.5) {
      lastBufferedEnd = bufferedEnd;
      lastBufferedCheck = now;
      return;
    }
    // Buffer not growing. If we're within 5s of the edge and it's been stagnant
    // for at least 20s, the stream is silently stuck.
    if (ahead < 5 && (now - lastBufferedCheck) > 20000) {
      triggerRecovery();
    }
  }, 10000);

  function clearStalledTimeout() {
    if (stalledTimeout) { clearTimeout(stalledTimeout); stalledTimeout = null; }
  }

  function cancelRecoveryAttempt() {
    if (recoveryCleanup) { recoveryCleanup(); recoveryCleanup = null; }
  }

  function triggerRecovery(options = {}) {
    const force = options.force === true;
    const immediate = options.immediate === true;
    if (isRecovering && !force) return;
    // Healthy-state guard: a spurious error fired while playback is fine
    // (e.g. Safari aborting an already-superseded range request) must not
    // tear down a perfectly working stream.
    if (!force && video.readyState >= 3 && !video.paused && !video.ended) return;
    // Capture play intent on the first trigger; subsequent triggers (during
    // repeated retries) must not overwrite it, because wasPlaying may have
    // been set to false by pause events emitted when video.src was changed.
    if (!autoplayIntent) autoplayIntent = wasPlaying;
    clearStalledTimeout();
    clearTimeout(retryTimeout);
    isRecovering = true;
    retryCount++;
    if (maxRetry === 0 || (maxRetry > 0 && retryCount > maxRetry)) {
      isRecovering = false;
      showVideoMessage(`Playback failed after ${retryCount - 1} retries. Reload the page to try again.`);
      return;
    }
    const label = maxRetry > 0 ? `${retryCount}/${maxRetry}` : `${retryCount}`;
    showVideoMessage(`Reconnecting… (attempt ${label})`);
    retryTimeout = setTimeout(recoverPlayback, immediate ? 0 : reconnectIntervalMs);
  }

  function recoverPlayback() {
    const targetTime     = savedPosition;
    // Use autoplayIntent, not wasPlaying — wasPlaying is unreliable here because
    // the browser fires pause when video.src is reassigned during recovery.
    const shouldAutoplay = autoplayIntent;
    // Reset buffer watchdog snapshot so it doesn't immediately re-fire after recovery
    lastBufferedEnd = 0;
    lastBufferedCheck = Date.now();
    // Detach any listeners/timers left over from a previous attempt
    cancelRecoveryAttempt();
    video.src = baseVideoUrl + "?t=" + Date.now();
    video.load();

    let settleTimer  = null;
    let attemptTimer = null;

    const cleanup = () => {
      video.removeEventListener("canplay", onCanPlay);
      video.removeEventListener("error",   onError);
      clearTimeout(settleTimer);
      clearTimeout(attemptTimer);
    };

    const fail = (immediate = false) => {
      cleanup();
      recoveryCleanup = null;
      isRecovering = false;
      triggerRecovery({ force: true, immediate: immediate === true });
    };

    const onCanPlay = () => {
      cleanup();
      recoveryCleanup = null;
      video.currentTime = targetTime;
      if (shouldAutoplay) {
        video.play()
          .then(() => { isRecovering = false; hideVideoMessage(); })
          // autoplayIntent cleared by the "playing" event listener on success
          .catch(fail);
      } else {
        isRecovering = false;
        autoplayIntent = false;
        hideVideoMessage();
      }
    };
    const onError = fail;

    recoveryCleanup = cleanup;
    video.addEventListener("canplay", onCanPlay, { once: true });
    // Settle window: Safari emits a transient `error` while swapping `video.src`.
    // Arming the recovery error listener immediately would re-trigger recovery on
    // that ghost event and ping-pong forever. Wait 1s before listening; if a real
    // error already landed inside the window (fast connection-refused while the
    // server is down), video.error is set — treat the attempt as failed instead
    // of waiting for an event that will never fire again.
    settleTimer = setTimeout(() => {
      if (!isRecovering) return;
      if (video.error) { fail(); return; }
      video.addEventListener("error", onError, { once: true });
    }, 1000);
    // A dead tunnel may leave the media request pending without an error event.
    // Keep attempts spaced by the configured reconnect interval in that case.
    attemptTimer = setTimeout(() => { if (isRecovering) fail(true); }, reconnectIntervalMs);
  }

  window.addEventListener("beforeunload", () => {
    clearTimeout(retryTimeout);
    clearStalledTimeout();
    cancelRecoveryAttempt();
    if (saveInterval) clearInterval(saveInterval);
    clearInterval(bufferWatchdog);
    if (audioCtx) audioCtx.close();
    if (video.currentTime > 0 && !video.ended) {
      sendResumeBeacon(resumeApiPath, video.currentTime, video.duration);
    }
  });
}

function showVideoMessage(message) {
  let el = document.getElementById("video-retry-message");
  if (!el) {
    el = document.createElement("div");
    el.id        = "video-retry-message";
    el.className = "video-message";
    document.body.appendChild(el);
  }
  el.textContent = message;
  show(el);
}

function hideVideoMessage() {
  const el = document.getElementById("video-retry-message");
  if (el) hide(el);
}

// ═══════════════════════════════════════════════════════════════
// SECTION 18: RESILIENT AUDIO PLAYER
// ═══════════════════════════════════════════════════════════════

function setupResilientAudioPlayer() {
  const audio = document.getElementById("resilientAudio");
  if (!audio) return;

  const maxRetry = Number.isInteger(AppState.data.resilient_player_max_retry)
    ? AppState.data.resilient_player_max_retry : 10;
  const reconnectIntervalMs = Math.max(1,
    AppState.data.resilient_player_reconnect_interval ?? 10) * 1000;

  let retryCount       = 0;
  let retryTimeout     = null;
  let stalledTimeout   = null;
  let lastPlaybackTime = 0;
  let wasPlaying       = false;
  let isRecovering     = false;
  // Play intent preserved across recovery cycles — load() emits pause events
  // that corrupt wasPlaying, same as the video player.
  let autoplayIntent   = false;

  audio.addEventListener("timeupdate", () => {
    if (!isRecovering && audio.currentTime > 0) lastPlaybackTime = audio.currentTime;
  });
  audio.addEventListener("play", () => { wasPlaying = true; });
  audio.addEventListener("playing", () => {
    wasPlaying = true;
    isRecovering = false;
    autoplayIntent = false;
    retryCount = 0;
    clearStalled();
  });
  audio.addEventListener("pause", () => { wasPlaying = false; clearStalled(); });
  audio.addEventListener("error", () => {
    if (isRecovering) return;
    const err = audio.error;
    if (!err || err.code === MediaError.MEDIA_ERR_ABORTED) return;
    if (err.code === MediaError.MEDIA_ERR_DECODE ||
        err.code === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED) return;
    attemptRecovery();
  });
  audio.addEventListener("stalled", () => {
    if (!isRecovering && wasPlaying && !stalledTimeout) {
      stalledTimeout = setTimeout(() => {
        stalledTimeout = null;
        if (!isRecovering && wasPlaying) attemptRecovery();
      }, 15000);
    }
  });
  window.addEventListener("online", () => {
    if (autoplayIntent || wasPlaying) {
      autoplayIntent = true;
      retryCount = 0;
      clearTimeout(retryTimeout);
      isRecovering = false;
      attemptRecovery();
    }
  });

  function clearStalled() {
    if (stalledTimeout) { clearTimeout(stalledTimeout); stalledTimeout = null; }
  }

  function attemptRecovery() {
    if (isRecovering) return;
    if (!autoplayIntent) autoplayIntent = wasPlaying;
    retryCount++;
    if (maxRetry === 0 || (maxRetry > 0 && retryCount > maxRetry)) return;
    isRecovering = true;
    clearStalled();
    clearTimeout(retryTimeout);
    retryTimeout = setTimeout(() => {
      const saved = lastPlaybackTime;
      audio.load();
      let metaWatchdog = null;
      const onMeta = () => {
        clearTimeout(metaWatchdog);
        if (saved > 0) audio.currentTime = saved;
        if (autoplayIntent) {
          audio.play().catch(() => { isRecovering = false; attemptRecovery(); });
        } else {
          isRecovering = false;
        }
      };
      audio.addEventListener("loadedmetadata", onMeta, { once: true });
      metaWatchdog = setTimeout(() => {
        audio.removeEventListener("loadedmetadata", onMeta);
        isRecovering = false;
        attemptRecovery();
      }, 30000);
    }, reconnectIntervalMs);
  }
}

// ═══════════════════════════════════════════════════════════════
// SECTION 19: RAW VIDEO RESUME (native browser player)
// ═══════════════════════════════════════════════════════════════

// ── Resume API ───────────────────────────────────────────────

async function persistResumePosition(videoPath, position, duration) {
  let url = `${videoPath}?resume-save=${position}`;
  if (Number.isFinite(duration)) url += `&duration=${duration}`;
  const res = await fetch(url, { method: "GET" });
  await assertResOK(res);
}

async function saveResumePosition(videoPath, position, duration) {
  try {
    await persistResumePosition(videoPath, position, duration);
  } catch {}
}

async function loadResumePosition(videoPath) {
  try {
    const res = await fetch(`${videoPath}?resume-get`);
    if (res.ok) return await res.json();
    return null;
  } catch {
    return null;
  }
}

async function clearResumePosition(videoPath) {
  try {
    await fetch(`${videoPath}?resume-clear`, { method: "GET" });
  } catch {}
}

// Unload-safe resume save. sendBeacon issues a POST, which the server's
// @public handler rejects (GET/HEAD only) — use a keepalive GET instead.
function sendResumeBeacon(videoPath, position, duration) {
  let url = `${videoPath}?resume-save=${position}`;
  if (Number.isFinite(duration)) url += `&duration=${duration}`;
  try { fetch(url, { method: "GET", keepalive: true }).catch(() => {}); } catch {}
}

async function resetSelectedResume() {
  const d = AppState.data;
  const selectedItems = Array.from(AppState.selection)
    .map(i => d.paths[i])
    .filter(Boolean);
  if (!selectedItems.length || !d.playback_resume_enabled) return;

  const resetButton = UI.selBtnResumeReset;
  const wasDisabled = resetButton.disabled;

  resetButton.disabled = true;
  resetButton.setAttribute("aria-busy", "true");

  try {
    const mediaPaths = await collectSelectedMediaPaths(selectedItems);
    if (!mediaPaths.length) {
      alert("No media files found in the selected items");
      return;
    }

    for (const pathName of mediaPaths) {
      await persistResumePosition(buildMediaUrl(pathName), 0);
    }

    Array.from(AppState.selection).forEach(index => {
      if (isMediaItem(d.paths[index])) {
        setRowResumeValue(index, 0);
      }
    });
  } catch (err) {
    alert(`Failed to reset resume, ${err.message}`);
  } finally {
    resetButton.disabled = wasDisabled;
    resetButton.removeAttribute("aria-busy");
  }
}

// ═══════════════════════════════════════════════════════════════
// SECTION 20: BREADCRUMB
// ═══════════════════════════════════════════════════════════════

function addBreadcrumb(href, uri_prefix) {
  const $bc = UI.breadcrumb;
  let parts = href === "/" ? [""] : href.split("/");
  const len = parts.length;
  let path  = uri_prefix;

  for (let i = 0; i < len; i++) {
    const name = parts[i];
    if (i > 0) {
      if (!path.endsWith("/")) path += "/";
      path += encodeURIComponent(name);
    }
    const encoded = encodedStr(name);
    if (i === 0) {
      $bc.insertAdjacentHTML("beforeend",
        `<a href="${path}" title="Root"><svg width="16" height="16" viewBox="0 0 16 16"><path d="M6.5 14.5v-3.505c0-.245.25-.495.5-.495h2c.25 0 .5.25.5.5v3.5a.5.5 0 0 0 .5.5h4a.5.5 0 0 0 .5-.5v-7a.5.5 0 0 0-.146-.354L13 5.793V2.5a.5.5 0 0 0-.5-.5h-1a.5.5 0 0 0-.5.5v1.293L8.354 1.146a.5.5 0 0 0-.708 0l-6 6A.5.5 0 0 0 1.5 7.5v7a.5.5 0 0 0 .5.5h4a.5.5 0 0 0 .5-.5z"/></svg></a>`);
    } else if (i === len - 1) {
      $bc.insertAdjacentHTML("beforeend", `<b>${encoded}</b>`);
    } else {
      $bc.insertAdjacentHTML("beforeend", `<a href="${path}">${encoded}</a>`);
    }
    if (i !== len - 1) {
      $bc.insertAdjacentHTML("beforeend", `<span class="separator">/</span>`);
    }
  }
}

// ═══════════════════════════════════════════════════════════════
// SECTION 21: UTILITIES
// ═══════════════════════════════════════════════════════════════

// ── Storage helpers ──────────────────────────────────────────
// Every access is guarded: Safari private browsing and blocked site data make
// the accessor itself throw SecurityError, which would abort player setup.

/**
 * @param {Storage} store
 * @param {string}  key
 * @returns {string|null}
 */
function storageGet(store, key) {
  try { return store.getItem(key); } catch { return null; }
}

/**
 * @param {Storage} store
 * @param {string}  key
 * @param {string}  value
 * @returns {boolean} whether the value was persisted
 */
function storageSet(store, key, value) {
  try { store.setItem(key, value); return true; } catch { return false; }
}

// ── DOM helpers ──────────────────────────────────────────────

const show = (...els) => els.forEach(el => el?.classList.remove("is-hidden"));
const hide = (...els) => els.forEach(el => el?.classList.add("is-hidden"));

// ── URL helpers ──────────────────────────────────────────────

function newUrl(name) {
  let url = baseUrl();
  if (!url.endsWith("/")) url += "/";
  return url + name.split("/").map(encodeURIComponent).join("/");
}

function baseUrl() {
  return location.href.split(/[?#]/)[0];
}

function baseName(url) {
  return decodeURIComponent(url.split("/").filter(v => v.length > 0).slice(-1)[0]);
}

function extName(filename) {
  const i = filename.lastIndexOf(".");
  if (i === -1 || i === 0 || i === filename.length - 1) return "";
  return filename.substring(i).toLowerCase();
}

// ── Formatting helpers ───────────────────────────────────────

function formatMtime(mtime) {
  if (!mtime) return "";
  const d = new Date(mtime);
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function pad(v) { return String(v).padStart(2, "0"); }

function formatDirSize(size) {
  const [num, unit] = formatDirSizeParts(size);
  return `${num} ${unit}`;
}

function formatDirSizeParts(size) {
  const unit = size === 1 ? "item" : "items";
  const num  = size >= MAX_SUBPATHS_COUNT ? `>${MAX_SUBPATHS_COUNT - 1}` : `${size}`;
  return [num, unit];
}

function formatFileSize(size) {
  if (size == null) return [0, "B"];
  if (size === 0)   return [0, "B"];
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i     = Math.floor(Math.log(size) / Math.log(1024));
  const ratio = i >= 3 ? 100 : 1;
  return [Math.round(size * ratio / Math.pow(1024, i)) / ratio, sizes[i]];
}

function formatDuration(seconds) {
  seconds = Math.ceil(seconds);
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds - h * 3600) / 60);
  const s = seconds - h * 3600 - m * 60;
  return `${pad(h)}:${pad(m)}:${pad(s)}`;
}

function formatPercent(pct) {
  return pct > 10 ? pct.toFixed(1) + "%" : pct.toFixed(2) + "%";
}

function formatTime(seconds) {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
}

function formatResumeHoursMinutes(seconds) {
  const totalSeconds = Math.max(0, Math.floor(seconds));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const secs = totalSeconds % 60;
  return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
}

// ── String helpers ───────────────────────────────────────────

function encodedStr(raw) {
  return raw.replace(/[\u00A0-\u9999<>&"]/g, c => "&#" + c.charCodeAt(0) + ";");
}

function escapeForOnclick(value) {
  // `&` MUST be escaped first. The HTML parser decodes entities inside the
  // attribute before the JS is parsed, so a file named `a&#39;+alert(1)+&#39;.txt`
  // would otherwise close the string literal and execute — stored XSS.
  return value
    .replace(/&/g, "&amp;")
    .replace(/\\/g, "\\\\")
    .replace(/'/g, "\\'")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

// Decode base64 → UTF-8 string (handles Unicode correctly)
function decodeBase64(b64) {
  const bin  = atob(b64);
  const len  = bin.length;
  const bytes = new Uint8Array(len);
  const arr  = new Uint32Array(bytes.buffer, 0, Math.floor(len / 4));
  let i = 0;
  for (; i < arr.length; i++) {
    arr[i] = bin.charCodeAt(i*4) |
             (bin.charCodeAt(i*4+1) << 8) |
             (bin.charCodeAt(i*4+2) << 16) |
             (bin.charCodeAt(i*4+3) << 24);
  }
  for (i = i * 4; i < len; i++) bytes[i] = bin.charCodeAt(i);
  return new TextDecoder().decode(bytes);
}

// ── Network helpers ──────────────────────────────────────────

async function assertResOK(res) {
  if (!(res.status >= 200 && res.status < 300)) {
    throw new Error((await res.text()) || `Invalid status ${res.status}`);
  }
}

function getEncoding(contentType) {
  const charset = contentType?.split(";")[1];
  if (/charset/i.test(charset)) {
    const enc = charset.split("=")[1];
    if (enc) return enc.toLowerCase();
  }
  return "utf-8";
}

// ── Clipboard ────────────────────────────────────────────────

function copyTextToClipboard(btn, text) {
  const showSuccess = () => {
    if (!btn) return;
    const orig  = btn.innerHTML;
    const title = btn.getAttribute("title");
    btn.innerHTML = CHECK_ICON_SUCCESS;
    btn.setAttribute("title", "Copied!");
    setTimeout(() => {
      btn.innerHTML = orig;
      if (title) btn.setAttribute("title", title);
      else       btn.removeAttribute("title");
    }, 1500);
  };

  const fallback = () => {
    try {
      const el = document.createElement("input");
      el.value = text;
      document.body.appendChild(el);
      el.select();
      document.execCommand("copy"); // eslint-disable-line -- fallback for older browsers
      document.body.removeChild(el);
      showSuccess();
    } catch {
      alert("Clipboard not supported");
    }
  };

  if (!btn) { fallback(); return; }

  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).then(showSuccess).catch(fallback);
  } else {
    fallback();
  }
}

// ── SVG icon helper ──────────────────────────────────────────

function getPathSvg(path_type) {
  switch (path_type) {
    case "Dir":        return ICONS.dir;
    case "SymlinkFile": return ICONS.symlinkFile;
    case "SymlinkDir":  return ICONS.symlinkDir;
    default:            return ICONS.file;
  }
}

// ═══════════════════════════════════════════════════════════════
// SECTION 22: PLAYLIST PLAYER
// ═══════════════════════════════════════════════════════════════

async function startPlaylist() {
  const d = AppState.data;
  const selectedItems = Array.from(AppState.selection)
    .map(i => d.paths[i])
    .filter(Boolean);
  if (!selectedItems.length) return;

  const playlistButton = UI.selBtnPlaylist;
  const wasDisabled = playlistButton.disabled;

  playlistButton.disabled = true;
  playlistButton.setAttribute("aria-busy", "true");

  try {
    const mediaPaths = await collectSelectedMediaPaths(selectedItems);
    const playlist = mediaPaths.map(buildPlaylistEntry);

    if (!playlist.length) {
      alert("No media files found in the selected items");
      return;
    }

    storageSet(sessionStorage, PLAYLIST_KEY, JSON.stringify({
      files: playlist,
      currentIndex: 0,
      mode: "sequential",
      resumeOnLoad: false
    }));

    clearSelection();
    window.location.href = playlist[0].url + "?view&playlist";
  } catch (err) {
    alert(`Failed to build playlist, ${err.message}`);
  } finally {
    playlistButton.disabled = wasDisabled;
    playlistButton.removeAttribute("aria-busy");
  }
}

function setupPlaylistPlayer(currentUrl) {
  let plData;
  try {
    plData = JSON.parse(storageGet(sessionStorage, PLAYLIST_KEY));
    if (!plData || !plData.files || !plData.files.length) throw new Error("No data");
  } catch {
    UI.notEditable.insertAdjacentHTML("afterend",
      `<p class="empty-notice">Playlist data not found. Go back and select files.</p>`);
    show(UI.notEditable);
    return;
  }

  const files = plData.files;
  let currentIndex = plData.currentIndex || 0;
  let currentMode = plData.mode || "sequential";
  let resumeOnLoad = plData.resumeOnLoad === true;
  const resumeEnabled = AppState.data.playback_resume_enabled === true;
  const fallbackTrackUrl = currentUrl.split(/[?#]/)[0];

  const plHTML = buildPlaylistHTML(files, currentIndex, currentMode);
  UI.notEditable.insertAdjacentHTML("afterend", plHTML);

  const video = document.getElementById("playlistVideo");
  const trackInfo = document.getElementById("playlist-track-info");
  const prevBtn = document.getElementById("playlist-prev");
  const nextBtn = document.getElementById("playlist-next");
  const panel = document.getElementById("playlist-panel");
  const backdrop = document.getElementById("playlist-backdrop");
  const toggleBtn = document.getElementById("playlist-toggle");
  const fileListEl = document.getElementById("playlist-files");
  const countEl = document.getElementById("playlist-count");
  setupAutoLetterboxFit(video);
  setupCustomVideoFullscreen({
    wrapper: video.closest(".playlist-wrapper"),
    button: document.getElementById("playlist-fullscreen"),
    toolbar: document.querySelector(".playlist-toolbar"),
    panel,
    backdrop,
  });

  // Same reconnection budget as the resilient player (server-supplied config)
  const maxRetry = Number.isInteger(AppState.data.resilient_player_max_retry)
    ? AppState.data.resilient_player_max_retry : 30;
  const reconnectIntervalMs = Math.max(1,
    AppState.data.resilient_player_reconnect_interval ?? 10) * 1000;

  let wasPlaying = false;
  let retryCount = 0;
  let retryTimeout = null;
  let stalledTimer = null;
  let bufferWatchdog = null;
  let isRecovering = false;
  let autoplayIntent = false;
  let autoPaused = false;
  let savedPosition = 0;
  let recoveryCleanup = null;
  let suppressResumeSave = false;
  let loadSequence = 0;
  let lastBufferedEnd = 0;
  let lastBufferedCheck = 0;
  let syncPlaylistBoost = () => {};
  // Guards against endless skip-cycling when every file in the list is broken
  let consecutiveSkips = 0;

  countEl.textContent = `${files.length} file${files.length !== 1 ? "s" : ""}`;

  function updatePlaylistStorage() {
    storageSet(sessionStorage, PLAYLIST_KEY, JSON.stringify({
      files, currentIndex, mode: currentMode, resumeOnLoad
    }));
  }

  function getTrackUrl(index = currentIndex) {
    return files[index]?.url || fallbackTrackUrl;
  }

  function markCurrentTrackResumable() {
    if (!resumeEnabled || resumeOnLoad) return;
    resumeOnLoad = true;
    updatePlaylistStorage();
  }

  function resetBufferWatchdog() {
    lastBufferedEnd = 0;
    lastBufferedCheck = Date.now();
  }

  function clearStalled() {
    if (stalledTimer) { clearTimeout(stalledTimer); stalledTimer = null; }
  }

  function cancelRecoveryAttempt() {
    if (recoveryCleanup) { recoveryCleanup(); recoveryCleanup = null; }
  }

  function loadFile(index, options = {}) {
    if (index < 0 || index >= files.length) return;
    const restoreResume = options.restoreResume === true;
    const loadId = ++loadSequence;
    currentIndex = index;
    resumeOnLoad = restoreResume;
    const file = files[index];
    // Track change aborts any recovery in progress and resets its state
    cancelRecoveryAttempt();
    clearTimeout(retryTimeout);
    clearStalled();
    suppressResumeSave = true;
    isRecovering = false;
    autoplayIntent = false;
    autoPaused = false;
    savedPosition = 0;
    retryCount = 0;
    resetBufferWatchdog();
    hideVideoMessage();
    // No cache-buster here. A track change is an ordinary navigation, and a
    // unique URL means the browser can never reuse what it already has: going
    // back one track, or round again in shuffle, re-downloaded the whole file
    // every time. Measured in Chromium over the real page: 4.47MB per
    // revisited track, zero cache hits. The cache-buster belongs in
    // recoverPlayback(), where forcing a fresh connection is the point.
    video.src = file.url;
    video.load();
    trackInfo.textContent = `${index + 1}/${files.length} - ${file.name}`;
    fileListEl.querySelectorAll(".pl-file").forEach((li, i) => {
      li.classList.toggle("active", i === index);
    });
    updatePlaylistStorage();
    syncPlaylistBoost();

    if (!restoreResume || !resumeEnabled) return;

    loadResumePosition(getTrackUrl(index)).then(resumeData => {
      if (!resumeData || resumeData.position <= 0) return;
      const resumePos = Math.max(0, resumeData.position - 10);
      const applyResume = () => {
        if (loadSequence !== loadId || currentIndex !== index || isRecovering) return;
        savedPosition = resumePos;
        video.currentTime = resumePos;
        showVideoMessage(`Resumed from ${formatTime(resumePos)}`);
        setTimeout(hideVideoMessage, 2000);
      };
      if (video.readyState >= 3) {
        applyResume();
      } else {
        const onCanPlay = () => {
          video.removeEventListener("canplay", onCanPlay);
          applyResume();
        };
        video.addEventListener("canplay", onCanPlay, { once: true });
      }
    }).catch(() => {});
  }

  function getNextIndex() {
    if (currentMode === "shuffle") {
      if (files.length === 1) return 0;
      let next;
      do { next = Math.floor(Math.random() * files.length); } while (next === currentIndex);
      return next;
    }
    return (currentIndex + 1) % files.length;
  }

  function playNext() {
    loadFile(getNextIndex(), { restoreResume: false });
    video.play().catch(() => {});
  }

  function playPrev() {
    loadFile(currentIndex === 0 ? files.length - 1 : currentIndex - 1, { restoreResume: false });
    video.play().catch(() => {});
  }

  // Render file list
  files.forEach((file, i) => {
    const li = document.createElement("li");
    li.className = "pl-file" + (i === currentIndex ? " active" : "");
    li.innerHTML = `<span class="pl-file-idx">${i + 1}</span><span class="pl-file-name">${encodedStr(file.name)}</span>`;
    li.addEventListener("click", () => { loadFile(i, { restoreResume: false }); video.play().catch(() => {}); });
    fileListEl.appendChild(li);
  });

  // Load initial file
  syncPlaylistBoost = setupPlaylistBoost(video, () => getTrackUrl());
  loadFile(currentIndex, { restoreResume: resumeEnabled && resumeOnLoad });
  syncPlaylistBoost();

  const saveInterval = setInterval(() => {
    if (!resumeEnabled || suppressResumeSave || isRecovering) return;
    if (video.currentTime > 0 && !video.paused && !video.ended) {
      markCurrentTrackResumable();
      saveResumePosition(getTrackUrl(), video.currentTime, video.duration);
    }
  }, 5000);

  // Video events
  video.addEventListener("timeupdate", () => {
    if (!isRecovering && !suppressResumeSave && video.currentTime > 0) {
      savedPosition = video.currentTime;
      markCurrentTrackResumable();
    }
  });
  video.addEventListener("play", () => { wasPlaying = true; autoPaused = false; });
  video.addEventListener("loadedmetadata", () => {
    suppressResumeSave = false;
    resetBufferWatchdog();
  });
  // Reset retryCount only when playback is actually running — the `play` event
  // fires on every play() call even if the attempt then fails, which would
  // defeat the retry cap.
  video.addEventListener("playing", () => {
    wasPlaying = true;
    suppressResumeSave = false;
    isRecovering = false;
    autoplayIntent = false;
    autoPaused = false;
    retryCount = 0;
    consecutiveSkips = 0;
    clearStalled();
    hideVideoMessage();
  });
  video.addEventListener("pause", () => {
    autoPaused = document.hidden && wasPlaying;
    wasPlaying = false;
    clearStalled();
    if (!resumeEnabled || suppressResumeSave || isRecovering) return;
    if (video.currentTime > 0 && !video.ended) {
      markCurrentTrackResumable();
      saveResumePosition(getTrackUrl(), video.currentTime, video.duration);
    }
  });
  video.addEventListener("waiting", () => {
    if (!isRecovering && wasPlaying && !video.ended) autoplayIntent = true;
  });

  video.addEventListener("ended", () => {
    resumeOnLoad = false;
    updatePlaylistStorage();
    if (resumeEnabled) clearResumePosition(getTrackUrl());
    if (currentMode === "single") {
      video.currentTime = 0;
      video.play().catch(() => {});
    } else {
      playNext();
    }
  });

  video.addEventListener("error", () => {
    suppressResumeSave = false;
    if (isRecovering) return;
    const err = video.error;
    if (!err || err.code === MediaError.MEDIA_ERR_ABORTED) return;
    if (err.code === MediaError.MEDIA_ERR_DECODE ||
        err.code === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED) {
      // Broken/unsupported file must not block the playlist — skip it
      consecutiveSkips++;
      if (files.length > 1 && currentMode !== "single" && consecutiveSkips < files.length) {
        playNext();
        showVideoMessage("Unplayable file, skipped to next.");
        setTimeout(hideVideoMessage, 3000);
      } else {
        showVideoMessage("Playback error: media cannot be decoded.");
      }
      return;
    }
    triggerRecovery();
  });

  video.addEventListener("stalled", () => {
    if (!isRecovering && wasPlaying && !stalledTimer) {
      autoplayIntent = true;
      stalledTimer = setTimeout(() => {
        stalledTimer = null;
        if (!isRecovering && wasPlaying) triggerRecovery();
      }, 15000);
    }
  });

  bufferWatchdog = setInterval(() => {
    if (suppressResumeSave || !wasPlaying || isRecovering || video.paused || video.ended) return;
    const buf = video.buffered;
    if (!buf || buf.length === 0) return;
    const bufferedEnd = buf.end(buf.length - 1);
    const ahead = bufferedEnd - video.currentTime;
    const now = Date.now();
    if (bufferedEnd > lastBufferedEnd + 0.5) {
      lastBufferedEnd = bufferedEnd;
      lastBufferedCheck = now;
      return;
    }
    if (ahead < 5 && (now - lastBufferedCheck) > 20000) {
      triggerRecovery();
    }
  }, 10000);

  window.addEventListener("online", () => {
    if (autoplayIntent || wasPlaying || isRecovering) {
      const forceRecovery = autoplayIntent || isRecovering;
      if (!forceRecovery && video.readyState >= 3 && !video.paused && !video.ended) return;
      const shouldAutoplay = autoplayIntent || wasPlaying;
      autoplayIntent = shouldAutoplay;
      retryCount = 0;
      clearTimeout(retryTimeout);
      cancelRecoveryAttempt();
      isRecovering = false;
      triggerRecovery({ force: forceRecovery, immediate: forceRecovery });
    }
  });

  document.addEventListener("visibilitychange", () => {
    if (document.hidden || isRecovering) return;
    if (!(wasPlaying || autoPaused)) return;
    autoPaused = false;
    if (video.readyState >= 3 && video.paused && !video.ended) {
      video.play().catch(() => {
        autoplayIntent = true;
        retryCount = 0;
        triggerRecovery();
      });
    } else if (video.paused || video.readyState < 3) {
      autoplayIntent = true;
      retryCount = 0;
      triggerRecovery();
    }
  });

  function triggerRecovery(options = {}) {
    const force = options.force === true;
    const immediate = options.immediate === true;
    if (isRecovering && !force) return;
    if (!force && video.readyState >= 3 && !video.paused && !video.ended) return;
    if (!autoplayIntent) autoplayIntent = wasPlaying;
    clearStalled();
    clearTimeout(retryTimeout);
    isRecovering = true;
    retryCount++;
    if (maxRetry === 0 || (maxRetry > 0 && retryCount > maxRetry)) {
      isRecovering = false;
      showVideoMessage(`Playback failed after ${retryCount - 1} retries. Reload the page to try again.`);
      return;
    }
    const label = maxRetry > 0 ? `${retryCount}/${maxRetry}` : `${retryCount}`;
    showVideoMessage(`Reconnecting… (attempt ${label})`);
    retryTimeout = setTimeout(recoverPlayback, immediate ? 0 : reconnectIntervalMs);
  }

  function recoverPlayback() {
    const targetTime     = savedPosition;
    const shouldAutoplay = autoplayIntent;
    cancelRecoveryAttempt();
    suppressResumeSave = true;
    resetBufferWatchdog();
    video.src = files[currentIndex].url + "?t=" + Date.now();
    video.load();

    let settleTimer  = null;
    let attemptTimer = null;

    const cleanup = () => {
      video.removeEventListener("canplay", onCanPlay);
      video.removeEventListener("error",   onError);
      clearTimeout(settleTimer);
      clearTimeout(attemptTimer);
    };

    const fail = (immediate = false) => {
      cleanup();
      recoveryCleanup = null;
      isRecovering = false;
      triggerRecovery({ force: true, immediate: immediate === true });
    };

    const onCanPlay = () => {
      cleanup();
      recoveryCleanup = null;
      // Restore position — a network drop mid-file must not restart from zero
      if (targetTime > 0) video.currentTime = targetTime;
      if (shouldAutoplay) {
        video.play()
          .then(() => { isRecovering = false; hideVideoMessage(); })
          .catch(fail);
      } else {
        isRecovering = false;
        autoplayIntent = false;
        hideVideoMessage();
      }
    };
    const onError = fail;

    recoveryCleanup = cleanup;
    video.addEventListener("canplay", onCanPlay, { once: true });
    // Settle window (see resilient player): swallow the transient src-swap error,
    // but catch a real error that landed inside the window via video.error.
    settleTimer = setTimeout(() => {
      if (!isRecovering) return;
      if (video.error) { fail(); return; }
      video.addEventListener("error", onError, { once: true });
    }, 1000);
    attemptTimer = setTimeout(() => { if (isRecovering) fail(true); }, reconnectIntervalMs);
  }

  // Navigation buttons
  prevBtn.addEventListener("click", playPrev);
  nextBtn.addEventListener("click", playNext);

  document.addEventListener("keydown", (e) => {
    if (e.shiftKey && e.key === "N") { e.preventDefault(); playNext(); }
    if (e.shiftKey && e.key === "P") { e.preventDefault(); playPrev(); }
  });

  // Mode buttons
  document.querySelectorAll(".playlist-mode-btn").forEach(btn => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".playlist-mode-btn").forEach(b => b.classList.remove("active"));
      btn.classList.add("active");
      currentMode = btn.dataset.mode;
      updatePlaylistStorage();
    });
  });

  // Sidebar toggle
  toggleBtn.addEventListener("click", () => {
    const open = panel.classList.toggle("panel-open");
    toggleBtn.title = open ? "Hide playlist" : "Show playlist";
    backdrop.classList.toggle("is-hidden", !open);
  });
  backdrop.addEventListener("click", () => {
    panel.classList.remove("panel-open");
    backdrop.classList.add("is-hidden");
    toggleBtn.title = "Show playlist";
  });

  // Cleanup
  window.addEventListener("beforeunload", () => {
    clearTimeout(retryTimeout);
    clearStalled();
    cancelRecoveryAttempt();
    clearInterval(saveInterval);
    clearInterval(bufferWatchdog);
    if (resumeEnabled && video.currentTime > 0 && !video.ended) {
      resumeOnLoad = true;
      updatePlaylistStorage();
      sendResumeBeacon(getTrackUrl(), video.currentTime, video.duration);
    } else {
      resumeOnLoad = false;
      updatePlaylistStorage();
    }
  });
}

function buildPlaylistHTML(files, currentIndex, mode) {
  const makeModeBtn = (m, label) =>
    `<button class="playlist-mode-btn${mode === m ? " active" : ""}" data-mode="${m}">${label}</button>`;

  return `
<div class="playlist-wrapper">
  <div class="playlist-toolbar">
    <div class="playlist-toolbar-left">
      <button id="playlist-prev" class="playlist-nav-btn" title="Previous (Shift+P)">${ICONS.prevTrack}</button>
      <span id="playlist-track-info" class="playlist-track-info">${currentIndex + 1}/${files.length} - ${encodedStr(files[currentIndex].name)}</span>
      <button id="playlist-next" class="playlist-nav-btn" title="Next (Shift+N)">${ICONS.nextTrack}</button>
    </div>
    <div class="playlist-toolbar-center">
      <span class="video-toolbar-label">Mode</span>
      ${makeModeBtn("sequential", "Seq")}
      ${makeModeBtn("shuffle", "Shuffle")}
      ${makeModeBtn("single", "Repeat")}
    </div>
    <div class="playlist-toolbar-right">
      <div class="boost-btns">
        <button class="boost-btn active" data-gain="1.0">Off</button>
        <button class="boost-btn" data-gain="1.4">+40%</button>
        <button class="boost-btn" data-gain="1.8">+80%</button>
        <button class="boost-btn" data-gain="2.2">+120%</button>
        <button class="boost-btn" data-gain="2.6">+160%</button>
        <button class="boost-btn" data-gain="3.0">+200%</button>
      </div>
      <button id="playlist-fullscreen" class="fullscreen-btn" type="button"
        title="Enter fitted fullscreen" aria-label="Enter fitted fullscreen"
        aria-controls="playlistVideo" aria-pressed="false">${ICONS.enterFullscreen}</button>
      <button id="playlist-toggle" class="playlist-nav-btn" title="Show playlist">${ICONS.listIcon}</button>
    </div>
  </div>
  <div class="playlist-body">
    <div class="playlist-player-col">
      <div class="video-container">
        <video id="playlistVideo" controls preload="auto"></video>
      </div>
    </div>
    <div id="playlist-panel" class="playlist-panel">
      <div class="playlist-panel-header">
        <span>Playlist</span>
        <span id="playlist-count" class="playlist-count"></span>
      </div>
      <ul id="playlist-files" class="playlist-files"></ul>
    </div>
  </div>
  <div id="playlist-backdrop" class="playlist-backdrop is-hidden"></div>
</div>`;
}

function setupPlaylistBoost(video, getTrackUrl) {
  let audioCtx = null;
  let gainNode = null;
  let pendingBoostApply = null;
  const btns = document.querySelectorAll(".playlist-toolbar .boost-btn");

  function getBoostKey() {
    return "sdufs_boost:" + getTrackUrl();
  }

  function readSavedBoost() {
    return parseFloat(storageGet(localStorage, getBoostKey())) || 1.0;
  }

  function setActiveBoostButton(gain) {
    btns.forEach(btn => btn.classList.toggle("active", parseFloat(btn.dataset.gain) === gain));
  }

  function clearPendingBoostApply() {
    if (!pendingBoostApply) return;
    video.removeEventListener("play", pendingBoostApply);
    pendingBoostApply = null;
  }

  function ensureChain() {
    if (audioCtx) { if (audioCtx.state === "suspended") audioCtx.resume(); return; }
    audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    const src = audioCtx.createMediaElementSource(video);
    gainNode = audioCtx.createGain();
    const comp = audioCtx.createDynamicsCompressor();
    comp.threshold.value = -6; comp.knee.value = 3; comp.ratio.value = 4;
    comp.attack.value = 0.005; comp.release.value = 0.1;
    src.connect(gainNode); gainNode.connect(comp); comp.connect(audioCtx.destination);
  }

  function setBoost(gain) {
    ensureChain();
    gainNode.gain.value = gain;
    storageSet(localStorage, getBoostKey(), String(gain));
    setActiveBoostButton(gain);
  }

  function syncStoredBoost() {
    const savedBoost = readSavedBoost();
    setActiveBoostButton(savedBoost);
    clearPendingBoostApply();
    if (audioCtx) {
      gainNode.gain.value = savedBoost;
      return;
    }
    if (savedBoost !== 1.0) {
      pendingBoostApply = () => {
        pendingBoostApply = null;
        setBoost(savedBoost);
      };
      video.addEventListener("play", pendingBoostApply, { once: true });
    }
  }

  btns.forEach(btn => {
    btn.addEventListener("click", () => {
      clearPendingBoostApply();
      setBoost(parseFloat(btn.dataset.gain));
    });
  });

  syncStoredBoost();

  window.addEventListener("beforeunload", () => { if (audioCtx) audioCtx.close(); }, { once: true });

  return syncStoredBoost;
}
