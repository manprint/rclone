// Loaded after index.js. Directories have no known entry count unless the
// server was started with --webui-dir-count; the server then reports size -1.
// Global function declarations are replaced by the later script and
// index.js only calls this at DOMContentLoaded, so the override applies.
function formatDirSizeParts(size) {
  if (size == null || size < 0) return ["—", ""];
  const unit = size === 1 ? "item" : "items";
  const num  = size >= MAX_SUBPATHS_COUNT ? `>${MAX_SUBPATHS_COUNT - 1}` : `${size}`;
  return [num, unit];
}
