use std::io::Stdout;

use crossterm::event::KeyCode;
use tui::{Frame, backend::{CrosstermBackend}};

pub trait RenderDialog {
    fn ui(&mut self, f: &mut Frame<CrosstermBackend<Stdout>>);
    fn process_input(&mut self, code: KeyCode);
    fn enter(&mut self) -> bool;
}