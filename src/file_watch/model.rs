use serde::{Deserialize, Serialize};
use super::schema::files_watches;

#[derive(Debug, Clone, Serialize, Deserialize, diesel::Queryable, diesel::Insertable)]
#[diesel(table_name=files_watches)]
pub struct FileWatch {
    pub file_id: String,
    pub watch_id: String,
}