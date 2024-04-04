CREATE TABLE IF NOT EXISTS tracks (
  id TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS playlists (
  id TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS playlist_tracks (
  playlist_id TEXT,
  track_id TEXT,
  added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (playlist_id, track_id),
  FOREIGN KEY (playlist_id) REFERENCES playlists(id),
  FOREIGN KEY (track_id) REFERENCES tracks(id)
);
