use serde::{Serialize, Deserialize};

use super::schema::watches;

#[derive(Debug, Clone, Serialize, Deserialize, diesel::Queryable, diesel::Insertable)]
#[diesel(table_name=watches)]
pub struct Watch {
    pub absolute_file_path: String,
    pub created_at: chrono::NaiveDateTime,
    pub default_expiration: Option<i32>,
}