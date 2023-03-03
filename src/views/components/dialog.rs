use std::io::Stdout;

use crossterm::event::KeyCode;
use tui::{backend::{CrosstermBackend}, Frame, widgets::{Block, Borders, Paragraph}, layout::{Layout, Direction, Constraint, Rect}, text::Span};

use crate::DbPool;

use super::{actions_dialog::{ActionsDialogState, ActionsDialog}, render_dialog::RenderDialog};

pub struct Dialog<T> {
    actions_state: ActionsDialogState,
    actions: Vec<String>,
    success_callback: Option<fn(DbPool, T)>,
    pool: DbPool,
    title: String,
    content: String,
    data: T
}

impl Dialog<String> {
    pub fn new<T>(pool: diesel::r2d2::Pool<diesel::r2d2::ConnectionManager<diesel::SqliteConnection>>, 
        title: String, 
        content: String,
        data: T) -> Dialog<T> {
        Dialog {
            title: title,
            actions: vec![String::from("OK"), String::from("Cancel")],
            actions_state: ActionsDialogState::default(),
            success_callback: None,
            pool: pool,
            content: content,
            data: data
        }
    }

    pub fn success_callback(mut self, callback: fn(DbPool, String)) -> Self {
        self.success_callback = Some(callback);
        self
    }

    pub fn next(&mut self) {
        if self.actions.len() == 0 {
            self.actions_state.select(None);
        }
        let i = match self.actions_state.selected() {
            Some(i) => {
                if i >= self.actions.len() - 1 {
                    0
                } else {
                    i + 1
                }
            }
            None => 0,
        };
        self.actions_state.select(Some(i));      

    }

    pub fn previous(&mut self) {
        if self.actions.len() == 0 {
            self.actions_state.select(None);
        }
        let i = match self.actions_state.selected() {
            Some(i) => {
                if i == 0 {
                    self.actions.len() - 1
                } else {
                    i - 1
                }
            }
            None => 0,
        };
        self.actions_state.select(Some(i));      
    }
}

fn centered_rect(percent_x: u16, percent_y: u16, r: Rect) -> Rect {
    let popup_layout = Layout::default()
        .direction(Direction::Vertical)
        .constraints(
            [
                Constraint::Percentage((100 - percent_y) / 2),
                Constraint::Percentage(percent_y),
                Constraint::Percentage((100 - percent_y) / 2),
            ]
            .as_ref(),
        )
        .split(r);

    Layout::default()
        .direction(Direction::Horizontal)
        .constraints(
            [
                Constraint::Percentage((100 - percent_x) / 2),
                Constraint::Percentage(percent_x),
                Constraint::Percentage((100 - percent_x) / 2),
            ]
            .as_ref(),
        )
        .split(popup_layout[1])[1]
}

impl RenderDialog for Dialog<String> {
    fn ui (&mut self, f: &mut Frame<CrosstermBackend<Stdout>>) {
        let block = Block::default().title(self.title.as_str()).borders(Borders::ALL);
        let area = centered_rect(40, 20, f.size());
        f.render_widget(block.clone(), area);
    
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Percentage(70),
                Constraint::Percentage(30)
            ].as_ref())
            .split(block.inner(area));

        let content = Paragraph::new(Span::raw(self.content.as_str()))
            .block(Block::default()
                .borders(Borders::NONE));
        
        f.render_widget(content, chunks[0]);
    
    
        let horizontal_list = ActionsDialog::default()
        .labels("OK", "Cancel");
    
        f.render_stateful_widget(horizontal_list, chunks[1], &mut self.actions_state)
    }

    fn enter(&mut self) -> bool {
        if self.actions_state.selected().is_none() {
            return false;
        }
        let i = match self.actions_state.selected() {
            Some(i) => i,
            None => 0
        };
        if i % 2 == 0 {
            match self.success_callback {
                Some(callback) => callback(self.pool.clone(), self.data.clone()),
                None => {}
            }
        }
        true
    }

    fn process_input(&mut self, code: KeyCode) {
        match code {
            KeyCode::Up => self.previous(),
            KeyCode::Down => self.next(),
            KeyCode::Left => self.previous(),
            KeyCode::Right => self.next(),
            _ => {}
        }
    }
}