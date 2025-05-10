pub mod db;
pub mod file;
pub mod file_event;
pub mod watch;
pub mod file_watch;
pub mod app;
pub mod views;

pub type DbPool = diesel::r2d2::Pool<diesel::r2d2::ConnectionManager<diesel::SqliteConnection>>;