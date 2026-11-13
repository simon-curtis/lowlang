# Islands

A tool window built with `std/ui/islands`: the window frame, controls, menus, a search picker, a confirm dialog and a theme that changes while the program runs.

## Run it

On Linux the program needs a Wayland session, a Vulkan 1.3 driver, `libvulkan.so.1`, `libwayland-client.so.0` and `libxkbcommon.so.0`. From the repository root:

```
low run docs/examples/islands/main.low
```

or build an executable first:

```
low build docs/examples/islands/main.low -o islands
./islands
```

For Windows, from any host:

```
low build docs/examples/islands/main.low -o islands.exe -target windows/amd64
```

The executable needs only the system libraries and the machine's Vulkan driver.

## What it shows

| Part | Where to look |
|---|---|
| A window that draws its own title bar, with the glow and grain behind it, made by `tool_window` and opened with `ui.Options { decorations: Client }` | `window_view`, `main` |
| A rail of buttons that switch between three views | `page_rail` |
| A tab strip with counts that filter the list | `people_tabs` |
| Rows with an avatar, a status pill and a checkbox | `person_row` |
| A drop-down that opens a search picker to choose the colour scheme, which changes every colour at once | `appearance_page`, `open_picker`, `layers` |
| A segmented control for the code size and another for the code face, with a preview | `size_segments`, `face_segments`, `code_sample` |
| A context menu on a right click of a row, whose danger item opens a confirm dialog | `row_menu`, `ask_to_delete`, `layers` |
| A side panel that slides open and shut with a `Slide` | `slide_to`, `step`, `detail_panel` |
| Key cap hints, and the chord they show bound to an action | `status_bar`, `start` |
| An empty state | `person_rows`, `inbox_page` |
| A status strip | `status_bar` |
| Icons the program draws itself, as `stroke_icon`s | `people_icon` and the functions after it |

## Keys and mouse

- `Ctrl+K` opens the colour scheme picker. Type to narrow it, Up and Down to move, Enter to choose and Escape to close.
- A right click on a row opens its menu. Up and Down move through it, Enter chooses and any other key closes it.
- A click on a row opens its details in the panel on the right; the button at the end of the tab strip and the cross in the panel close it.
- In the delete dialog, Enter deletes and Escape cancels.

## Theme

`theme_of` builds a `Theme` from three signals each time the window is drawn, and every function that depends on the scheme or the code face takes it first. Choosing a scheme in the picker sets the signal, and the next frame draws with the new accent, glow and code face.
