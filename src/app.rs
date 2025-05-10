use std::{time::{Duration, Instant}, io::{self, Stdout}};

use crossterm::event::{Event, self, KeyCode};
use diesel::SqliteConnection;
use tui::{widgets::TableState, backend::{CrosstermBackend}, Terminal};

use crate::{watch, views::{components::{watch_new_folder_dialog, unwatch_folder_dialog, render_dialog::RenderDialog}, self}, DbPool};

pub struct App<'a> {
    pub state: TableState,
    pub items: Vec<Vec<String>>,
    pub commands: Vec<Vec<&'a str>>,
    pub dialog: Option<Box<dyn RenderDialog>>
}

impl<'a> App<'a> {
    pub fn new() -> App<'a> {
        App {
            state: TableState::default(),
            items: Vec::new(),
            commands: vec![
                vec!["<q>", "Quit"],
                vec!["<a>", "Watch new folder"],
                vec!["<d>", "Unwatch folder"],
                vec!["<e>", "Edit expiration date"]
            ],
            dialog: None
        }
    }

    pub fn get_selected_item(&mut self) -> Option<Vec<String>> {
        if self.items.len() == 0 {
            return None
        }
        match self.state.selected() {
            Some(i) => Some(self.items[i].clone()),
            None => None
        }
    }

    pub fn next(&mut self) {
        if self.items.len() == 0 {
            self.state.select(None);
        }
        let i = match self.state.selected() {
            Some(i) => {
                if i >= self.items.len() - 1 {
                    0
                } else {
                    i + 1
                }
            }
            None => 0,
        };
        self.state.select(Some(i));
    }

    pub fn previous(&mut self) {
        if self.items.len() == 0 {
            self.state.select(None);
        }
        let i = match self.state.selected() {
            Some(i) => {
                if i == 0 {
                    self.items.len() - 1
                } else {
                    i - 1
                }
            }
            None => 0,
        };
        self.state.select(Some(i));
    }

    pub fn on_tick(&mut self, conn: &mut SqliteConnection) {
        self.items.clear();
        let watches = watch::service::list_watches(conn).unwrap();
        
        for watch in &watches {
            let data = watch.clone();
            let mut default_expiration = String::from("NULL");
            if data.default_expiration.is_some() {
                let default_expiration_dt = data.default_expiration.unwrap();
                default_expiration = default_expiration_dt.to_string();
            }
            self.items.push(vec![data.absolute_file_path, default_expiration, String::from("ACTIVE")]);
        }
    }
}

pub fn run_app(pool: DbPool, 
    terminal: &mut Terminal<CrosstermBackend<Stdout>>, 
    mut app: App,
    tick_rate: Duration) -> io::Result<()> {
    let mut last_tick = Instant::now();
    
    loop {
        terminal.draw(|f| views::main::ui(f, &mut app))?;

        if let Event::Key(key) = event::read()? {
            match app.dialog {
                None => match key.code {
                    KeyCode::Char('q') => return Ok(()),
                    KeyCode::Char('a') => app.dialog = Some(Box::new(watch_new_folder_dialog::create(pool.clone(), String::new()))),
                    KeyCode::Char('d') => app.dialog = {
                        match app.get_selected_item() {
                            Some(item) => Some(Box::new(unwatch_folder_dialog::create(pool.clone(), item[0].as_str()))),
                            None => None
                        }
                    },
                    KeyCode::Char('e') => app.dialog = {
                        match app.get_selected_item() {
                            Some(item) => Some(Box::new(watch_new_folder_dialog::create(pool.clone(), item[0].clone()))),
                            None => None
                        }
                    },
                    KeyCode::Down => app.next(),
                    KeyCode::Up => app.previous(),
                    _ => {}
                }
                Some(ref mut dialog) => match key.code {
                    KeyCode::Esc => app.dialog = None,
                    KeyCode::Enter => {
                        if dialog.enter() == true {
                            app.dialog = None
                        }
                    },
                    x => dialog.process_input(x)
                }
            }
        }
        if last_tick.elapsed() >= tick_rate {
            let mut conn = pool.get().unwrap();
            app.on_tick(&mut conn);
            last_tick = Instant::now();
        }
    }
}