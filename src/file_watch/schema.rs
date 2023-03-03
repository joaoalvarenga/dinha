diesel::table! {
    files_watches (file_id, watch_id) {
        file_id -> Text,
        watch_id -> Text,
    }
}