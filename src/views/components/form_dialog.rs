use std::io::Stdout;

use crossterm::event::KeyCode;
use tui::{widgets::{ListState, ListItem, List, Block, Borders}, backend::{CrosstermBackend}, Frame, layout::{Layout, Direction, Rect, Constraint}, style::{Style, Color, Modifier}, text::{Span, Spans}};
use unicode_width::UnicodeWidthStr;

use crate::DbPool;

use super::{actions_dialog::{ActionsDialog, ActionsDialogState}, render_dialog::RenderDialog};
pub enum InputMode {
    Normal,
    Editing
}

pub struct Field {
    pub id: String,
    pub label: String,
    pub value: String
}

pub struct FormDialog {
    title: String,
    actions_state: ActionsDialogState,
    actions: Vec<String>,
    fields: Vec<Field>,
    field_state: ListState,
    pub mode: InputMode,
    selected_element: usize,
    success_callback: Option<fn(DbPool, &Vec<Field>)>,
    pool: DbPool
}

impl FormDialog {
    pub fn new(pool: diesel::r2d2::Pool<diesel::r2d2::ConnectionManager<diesel::SqliteConnection>>) -> FormDialog {
        let mut state = ListState::default();
        state.select(Some(0));
        FormDialog {
            title: String::default(),
            fields: Vec::default(),
            actions: vec![String::from("OK"), String::from("Cancel")],
            actions_state: ActionsDialogState::default(),
            field_state: state,
            mode: InputMode::Normal,
            selected_element: 0,
            success_callback: None,
            pool: pool
        }
    }

    pub fn title(mut self, title: String) -> Self {
        self.title = title.clone();
        self
    }

    pub fn fields(mut self, fields: Vec<Field>) -> Self {
        self.fields = fields;
        self
    }

    pub fn success_callback(mut self, callback: fn(DbPool, &Vec<Field>)) -> Self {
        self.success_callback = Some(callback);
        self
    }

    pub fn insert_char_current(&mut self, c: char) {
        if self.field_state.selected().is_none() {
            return;
        }
        let i = match self.field_state.selected() {
            Some(i) => i,
            None => 0
        };
        self.fields[i].value.push(c);
    }

    pub fn remove_char_current(&mut self) {
        if self.field_state.selected().is_none() {
            return;
        }
        let i = match self.field_state.selected() {
            Some(i) => i,
            None => 0
        };
        self.fields[i].value.pop();
    }

    pub fn next(&mut self) {
        let i = self.selected_element + 1;
        if i >= self.fields.len() + self.actions.len() {
            self.selected_element = 0;
            self.actions_state.select(None);
            self.field_state.select(Some(0));
            self.mode = InputMode::Editing;
            return;
        }

        if i >= self.fields.len() {
            self.selected_element = i;
            self.actions_state.select(Some(self.selected_element));
            self.field_state.select(None);
            self.mode = InputMode::Normal;
            return;
        }

        self.selected_element = i;
        self.actions_state.select(None);
        self.field_state.select(Some(self.selected_element));
        self.mode = InputMode::Editing;        

    }

    pub fn previous(&mut self) {
        if self.selected_element as i32 - 1 < 0 {
            self.selected_element = self.fields.len() + self.actions.len() - 1;
            self.actions_state.select(Some(self.selected_element));
            self.field_state.select(None);
            self.mode = InputMode::Editing;
            return;
        }

        let i = self.selected_element - 1;
        if i >= self.fields.len() {
            self.selected_element = i;
            self.actions_state.select(Some(self.selected_element));
            self.field_state.select(None);
            self.mode = InputMode::Normal;
            return;
        }

        self.selected_element = i;
        self.actions_state.select(None);
        self.field_state.select(Some(self.selected_element));
        self.mode = InputMode::Editing; 

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

impl RenderDialog for FormDialog {
    fn ui(&mut self, f: &mut Frame<CrosstermBackend<Stdout>>) {
        let block = Block::default().title(self.title.clone()).borders(Borders::ALL);
        let area = centered_rect(40, 20, f.size());
        f.render_widget(block.clone(), area);
    
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Percentage(70),
                Constraint::Percentage(30)
            ].as_ref())
            .split(block.inner(area));
    
        let panels = Layout::default()
            .direction(Direction::Horizontal)
            .constraints([
                Constraint::Percentage(50),
                Constraint::Percentage(50)
            ].as_ref())
            .split(chunks[0]);
    
    
        let mut labels: Vec<ListItem> = Vec::new();
        let mut values: Vec<ListItem> = Vec::new();
    
        for field in &self.fields {
            let f = field.clone();
            let content = vec![Spans::from(Span::styled(f.label.as_str(), Style::default().fg(Color::Yellow).add_modifier(Modifier::BOLD)))];
            labels.push(ListItem::new(content));
    
            let content = vec![Spans::from(Span::raw(f.value.as_str()))];
            values.push(ListItem::new(content));
        }
    
        let labels =
            List::new(labels);
        f.render_widget(labels, panels[0]);
    
        let values =
            List::new(values)
            .highlight_style(
                Style::default()
                    .fg(Color::White)
                    .add_modifier(Modifier::BOLD),
            );
        f.render_stateful_widget(values, panels[1], &mut self.field_state);

        match self.mode {
            InputMode::Normal =>
                // Hide the cursor. `Frame` does this by default, so we don't need to do anything here
                {}
    
            InputMode::Editing => {
                let i = self.field_state.selected().unwrap();
                let w = self.fields[i].value.clone();
                
                // Make the cursor visible and ask tui-rs to put it at the specified coordinates after rendering
                f.set_cursor(
                    // Put cursor past the end of the input text
                    panels[1].x + w.width() as u16,
                    // Move one line down, from the border to the input line
                    panels[1].y + i as u16,
                )
            }
        }
    
        let horizontal_list = ActionsDialog::default()
        .labels("OK", "Cancel");
    
        f.render_stateful_widget(horizontal_list, chunks[1], &mut self.actions_state)
    }

    fn process_input(&mut self, code: KeyCode) {
        match code {
            KeyCode::Up => self.previous(),
            KeyCode::Down => self.next(),
            KeyCode::Left => self.previous(),
            KeyCode::Right => self.next(),
            x => match self.mode {
                InputMode::Editing => match x {
                    KeyCode::Char(c) => {
                        self.insert_char_current(c);
                    },
                    KeyCode::Backspace => {
                        self.remove_char_current();
                    },
                    _ => {}
                },
                _ => {}
            }
        }
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
                Some(callback) => callback(self.pool.clone(), &self.fields),
                None => {}
            }
        }
        true
    }

}