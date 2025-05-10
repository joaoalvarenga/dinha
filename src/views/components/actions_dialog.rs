use tui::{buffer::Buffer, widgets::{StatefulWidget, Paragraph, Block, Borders, Widget}, layout::{Rect, Layout, Constraint, Direction, Alignment}, text::{Span}, style::{Style, Color}};

#[derive(Default)]
pub struct ActionsDialog<'a> {
    confirm_label: &'a str,
    cancel_label: &'a str,
    block: Option<Block<'a>>,
}

#[derive(Debug, Clone, Default)]
pub struct ActionsDialogState {
    offset: usize,
    selected: Option<usize>,
}

impl<'a> ActionsDialog<'a> {
    pub fn labels(mut self, confirm_label: &'a str, cancel_label: &'a str) -> ActionsDialog<'a> {
        self.confirm_label = confirm_label;
        self.cancel_label = cancel_label;
        self
    }
    pub fn block(mut self, block: Block<'a>) -> Self {
        self.block = Some(block);
        self
    }
}

impl ActionsDialogState {
    pub fn selected(&self) -> Option<usize> {
        self.selected
    }

    pub fn select(&mut self, index: Option<usize>) {
        self.selected = index;
        if index.is_none() {
            self.offset = 0;
        }
    }
}

impl<'a> StatefulWidget for ActionsDialog<'a> {
    type State = ActionsDialogState;
    fn render(mut self, area: Rect, buf: &mut Buffer, state: &mut Self::State) {
        let table_area = match self.block.take() {
            Some(b) => {
                let inner_area = b.inner(area);
                b.render(area, buf);
                inner_area
            }
            None => area,
        };
        

        let rects = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(50), Constraint::Percentage(50)].as_ref())
        .horizontal_margin(3)
        .split(table_area);
    
        let confirm_paragraph = get_label_paragraph(0, self.confirm_label, state.selected).alignment(Alignment::Center);
        confirm_paragraph.render(rects[0], buf);

        let cancel_paragraph = get_label_paragraph(1, self.cancel_label, state.selected).alignment(Alignment::Center);
        cancel_paragraph.render(rects[1], buf);
    }
}

fn get_label_paragraph(index: usize, label: &str, selected: Option<usize>) -> Paragraph {
    let style = match selected {
        Some(b) => {
            if index == b % 2 {
                Style::default().fg(Color::Black).bg(Color::White)
            } else {
                Style::default()
            }
        }
        None => Style::default()
    };
    Paragraph::new(Span::styled(label, style))
        .block(Block::default()
            .borders(Borders::NONE))
    
}