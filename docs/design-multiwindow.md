# Several windows in one process

Status: design. Pull request 1 of section 6 is implemented in `std/ui`, pull request 2 in `std/ui/gpu` and pull requests 3 and 4 in `std/ui/platform`, and pull request 6 in `std/ui` and `std/ui/gpu` for Windows. On Wayland `open_window` fails with `Unsupported` until pull request 5.

## Goal

A `std/ui` program opens and runs several OS windows from one process. The case that drives the design is an application whose main window opens tool windows, such as a settings window, a compare window or a dashboard. Each tool window has its own title, size and taskbar presence, closes on its own, and reads and writes the same signals as the main window.

## Where the code stands

- `ui.open` returns one `App`, which holds a `platform.Window`, a `gpu.Gpu` with its swapchain, and a `Context` (`std/ui/ui.low:40-45`, `50-77`).
- `ui.run` loops over that single window. It calls `platform.next` on it, dispatches the events, drains the posts, fires the timers and presents one frame (`std/ui/ui.low:121-144`).
- `ui.run` computes one reveal deadline when it starts (`std/ui/ui.low:123`) and keeps one `revealed` flag per context (`std/ui/context.low:55`, `std/ui/ui.low:101-107`).
- `Context` is one `*State` (`std/ui/context.low:119-121`). That `State` mixes per-window state, such as hitboxes, focus, layers and size, with state that belongs to the whole application: the text system (`:51`), the keymap (`:102`), the posts channel and waker (`:110-111`), the clipboard closures (`:112-113`) and the clock (`:109`).
- A signal's cell points at one `*State` as its owner (`std/ui/signal.low:8-11`). `set` makes only that state dirty (`std/ui/signal.low:24-27`). `spawn_into` rebuilds a `Context` from that owner (`std/ui/task.low:21-25`).
- On Windows, `platform.open` takes `sync.lock_thread()` (`std/ui/platform/window_windows.low:43`), and `close_window` releases it (`:242`). The calls nest (`std/sync/thread.low:3`), so two windows opened on one green thread keep the lock balanced.
- The Windows pump is `PeekMessageW(null, …)` (`std/ui/platform/window_windows.low:188-196`), so it dispatches the messages of every window on the thread. The window procedure finds each window's state through `GWLP_USERDATA` (`std/ui/platform/messages_windows.low:6-21`), so every event reaches its own window's queue. The `win32_pair` fixture (`compiler/tests/ui/testdata/win32_pair/main.low`) relies on this behaviour.
- `next_events` waits only until its own window's queue is non-empty (`std/ui/platform/window_windows.low:140-156`). A loop that calls `next` on each window in turn therefore blocks on one window while another window has events waiting.
- On Wayland, each `platform.Window` opens its own `wl_display` connection, registry, seat, keyboard, xkb context and pump green thread (`std/ui/platform/window_linux.low:21-82`, `123-146`, `181-187`). The pointer and keyboard listeners ignore their `surface` argument (`std/ui/platform/input_linux.low:188-202`, `260-270`).
- `gpu.State` combines the instance, the device, one `VkSurfaceKHR`, one swapchain, the frame ring, the pipelines and the glyph atlas texture (`std/ui/gpu/gpu.low:13-28`). `open_for` builds all of these from one surface (`std/ui/gpu/swapchain.low:22-27`). The device is chosen against that surface only (`std/ui/gpu/device.low:27-41`, `72-89`).

Two `ui.open` calls give two Vulkan instances and devices, two text systems with separate atlases, and two contexts whose signals do not reach each other. And `ui.run` cannot drive both.

Related issues: `gh issue list -R simon-curtis/lowlang --search window` finds no open request for several windows. #37 (text input, clipboard, IME and window decorations) touches the Wayland decoration limit in section 5. #205 is the cloak-until-first-present change that section 5 extends to each window.

## 1. Public API

### What stays the same

`ui.Options`, `ui.open`, `ui.run`, `App.cx` and every function that takes a `Context` keep their signatures and behaviour. A program that opens one window does not change.

### Opening a tool window

```low
/// What `open_window` needs to make a window beside the application's others.
pub struct WindowOptions {
    pub title: str = "Low"
    /// The width and height of the content, in logical pixels.
    pub width: i64 = 800
    pub height: i64 = 600
    /// The smallest content size the user can resize the window to, in logical pixels. Zero means no limit.
    pub min_width: i64 = 0
    pub min_height: i64 = 0
    pub background: geometry.Color = geometry.rgb(0xffffff)
    pub titlebar: TitlebarOptions = TitlebarOptions {}
    /// The window this one belongs to. It stays above its owner, minimises with it and closes before it.
    pub owner: Window? = none
    /// Where the window first appears.
    pub placement: Placement = Placement.CentreOnOwner
    /// `true` gives the window its own taskbar button, even when it has an owner.
    pub taskbar: bool = true
    /// `false` shows the window without activating it, so it takes neither the keyboard focus nor the foreground from the program the user is working in. `platform.Options.activate` has the same field; `ui.Options` and this struct carry it through.
    pub activate: bool = true
    /// Runs on the UI green thread once the window has closed, with why it closed.
    pub closed: fn(CloseReason) = fn(why: CloseReason) {}
}

pub enum CloseReason {
    /// The user or the program closed it.
    Requested
    /// The error that closed it.
    Failed(Error)
}

pub enum Placement {
    /// Wherever the system puts a new window.
    System
    /// Centred on the owner, kept inside the work area of the owner's monitor. A window with no owner is centred on the main window.
    CentreOnOwner
    /// Centred on the work area of the monitor that holds the main window.
    CentreOnScreen
}

/// A copyable handle to an open window of the application.
pub struct Window { slot: *WindowSlot }

/// Opens a window that runs `render` in the application that `cx` belongs to, and returns once it exists. The window stays invisible until it presents its first frame.
pub fn open_window(cx: Context, o: WindowOptions, render: fn() -> Element) -> Window ! Error

/// Closes `w` and every window it owns, owned windows first, then runs each one's `closed` callback. Closing a window that is already closed does nothing.
pub fn close_window(w: Window)

/// Returns `true` until `w` has closed.
pub fn is_open(w: Window) -> bool

/// Returns the context of `w`'s user interface, for focus, scrolling, overlays and actions inside it.
pub fn window_context(w: Window) -> Context

/// Returns the main window, the one `ui.open` made.
pub fn main_window(cx: Context) -> Window

/// Raises `w`, gives it keyboard focus and restores it when it is minimised.
pub fn activate(w: Window)

/// Sets `w`'s title.
pub fn set_title(w: Window, title: str)
```

Rules:

- `open_window` runs on the UI green thread, which runs `ui.run`. Event handlers, posted closures, timers and task `done` callbacks all run on that thread. A background green thread opens a window by posting a closure (`ui.post(handle(cx), fn() { … })`, `std/ui/context.low:315-323`).
- `open_window` is synchronous. It creates the platform window hidden, makes its swapchain and registers it with the loop, so a failure returns as `Error`. If a render function calls it, it fails with `Error.Platform(Unsupported("open_window inside render"))`: `run` is iterating the window list at that point, and a render function has no side effects. `match` must cover every variant (`docs/design.md:608`), so the design adds no variant to `ui.Error` or `gpu.Error`.
- `open_window` works before `run` starts as well. The window waits, hidden, until `run` presents its first frame.
- `render` belongs to the window. `ui.run(app, render)` supplies the main window's render function, as it does today.
- The pattern "open settings, or raise it if it is open" keeps a `Signal[Window?]`, calls `activate` when `is_open` is true, and clears the signal in `closed`.

### Closing

- `CloseRequested` on a tool window closes that window, as `close_window` does. `CloseRequested` on the main window ends `run`, as it does today (`std/ui/context.low:866-867`, `std/ui/ui.low:124`). Before `run` returns, it closes every tool window, owned windows before their owners, and runs each `closed` callback.
- `quit(cx)` ends `run` whichever window `cx` belongs to. Its doc already says "ends `run`" (`std/ui/context.low:267-270`). The per-window `closed` flag that `CloseRequested` sets is kept apart from the application's `quit` flag, so `closed(cx)` reports the window and `quit` reports the application.
- Closing the main window is the only way the user ends `run`; tool windows alone never keep the process alive.
- Tool windows are never modal: no window disables its owner, and every open window takes input at once.
- After a window closes, its `Context` stays valid as a headless context. `frame` and `dispatch` still work on it, nothing is presented, and `is_open` is `false`.

### Render, focus, keys and overlays per window

- Each window has its own hitboxes, hover, pressed state, focus, scroll offsets, field and text-area state, drag, caret, cursor and layers. Keys go to the window that has the system's keyboard focus and follow that window's focus path (`std/ui/context.low:1067-1106`).
- The keymap is shared. `bind_keys` on any window's context binds for every window (`std/ui/actions.low:33-37`). Each window keeps its own pending chord (`pending_keys`, `pending_timer`, `std/ui/context.low:103-105`), so a chord started in one window does not finish in another.
- `dispatch_action(window_context(w), name)` sends an action down another window's focus path, for example to have a menu in the main window refresh the dashboard.
- Context menus, popovers, dropdowns, dialogs and tooltips are layers of the window that opened them (`std/ui/layer.low:12-27`, `93-118`). They stay inside its viewport (`std/ui/layer.low:279-296`). A dialog traps focus within its own window only (`std/ui/layer.low:207-216`).
- `pointer_cursor`, `text_input` and `window_control_area_at` answer for the window whose context they are given, as today (`std/ui/context.low:191-231`, `253-255`). The loop applies each window's caret and cursor to its own platform window (`std/ui/ui.low:88-99`).

### Signals across windows

A signal belongs to the application, not to one window. `signal(cx, v)` called with any window's context gives a cell that every window reads with `get` and writes with `set` or `update`.

- `Cell.owner` changes from the window's `*State` to the application's `*Shared` (`std/ui/signal.low:8-11`).
- `set` increments `Shared.generation` instead of setting one window's `dirty`. A window is due for a frame when its own `dirty` is set or when the generation it last rendered differs from `Shared.generation`. Setting a signal stays O(1).
- The cost of this rule: one `set` makes every open window render again. A tool window is a few hundred elements and is only presented when due, so the cost is bounded. Section 7 describes read tracking, which is the follow-up for when it is not.
- `spawn_into` and `background_spawn` take the application's handle from `Shared` (`std/ui/task.low:14-25`). A task started from a tool window that closes before the task finishes still delivers its result, because the posts channel and the waker do not belong to that window.

### Headless contexts

`headless(background)` returns a context whose `Shared` holds it alone, as today, so separate `headless` calls stay isolated. `open_window` on a headless context makes a second headless context that shares the first's `Shared`, including its signals, keymap, posts and manual clock, and returns its `Window`. A test drives each one with `frame(window_context(w), …)` and `dispatch(window_context(w), …)` (`std/ui/context.low:339`, `857`). `advance_clock` moves the shared manual clock and fires the timers of every window (`std/ui/clock.low:8-11`).

## 2. Threading

All windows run on one UI green thread, which is locked to one OS thread. There is no strong reason for a thread per window:

- Win32 sends a window's messages only to the thread that created it. One locked thread that pumps with `PeekMessageW(null)` (`std/ui/platform/window_windows.low:190`) already dispatches every window on it.
- Signals, fields and element closures are plain heap values without locks (`std/ui/signal.low:1-3`). Sharing them across OS threads would need synchronisation everywhere.
- Wayland runs one connection per client, with one event queue, which one dispatcher reads.

### The display

`std/ui/platform` gains a `Display`, which holds everything that belongs to the connection, not to one window:

```low
/// The connection to the windowing system that a program's windows share.
pub struct Display { pub handle: DisplayHandle  drop close_display_on(handle) }

/// A copyable view of a `Display`.
pub struct DisplayHandle { state: *DisplayState }

pub fn open_display() -> Display ! Error
pub fn open_in(d: Display, o: Options) -> Window ! Error
pub fn open_in_on(h: DisplayHandle, o: Options) -> Window ! Error
/// Waits until a window of `d` has events, a waker fires or `timeout_ms` passes.
pub fn wait(d: Display, timeout_ms: i64) ! Error
pub fn wait_on(h: DisplayHandle, timeout_ms: i64) ! Error
pub fn waker_of(d: Display) -> Waker
pub fn waker_of_on(h: DisplayHandle) -> Waker
```

Until pull request 5 gives Wayland a shared connection, `Display`, `DisplayHandle` and these functions are declared in `std/ui/platform/display_windows.low`, so they exist on Windows only and Linux keeps `platform.open` alone.

A droppable value cannot be held by a closure, a slice or a pointer, and `Display` and `Window` are droppable. The platform therefore follows the pattern of `std/file` and `std/net`: the owner is droppable and has a public copyable `handle`, and each operation has an `_on` form that takes the handle. Pull request 6 keeps `DisplayHandle` in the shared state that `Shared.open` and its closure reach, and keeps the owning `Display` where `ui.run` binds it. Releasing the display marks its state closed, so `open_in_on` and `wait_on` on any copy of the handle fail with `Unsupported`, and a waker taken from it does nothing. `WindowHandle` is the same for windows and arrives with pull request 4; the platform views of both handles land in pull requests 3 and 4, the GPU views in pull request 6.

```low
/// An open window. Releasing it closes it, and closes the display when `open` made the window.
pub struct Window { pub handle: WindowHandle  detached: bool  drop { if !detached { close_on(handle) } } }

/// A copyable view of a window, for `Options.owner` and for code that cannot hold the `Window`.
pub struct WindowHandle { state: *WindowState }

pub fn adopt(h: WindowHandle) -> Window
pub fn detach(mut w: Window) -> WindowHandle
pub fn close_on(h: WindowHandle)
pub fn next_on(h: WindowHandle, timeout_ms: i64) -> Event[] ! Error
pub fn native_on(h: WindowHandle) -> Native
pub fn set_title_on(h: WindowHandle, title: str)
```

`show_on`, `waker_on`, `clipboard_on`, `set_text_input_on`, `set_cursor_on`, `set_hit_test_on` and `controls_on` complete the set, one `_on` form for each operation on a `Window`. The closed state lives in the shared `*WindowState`, so every copy sees it: after `close_on`, or after the `Window` is released, `next_on` and the clipboard calls fail with `Unsupported`, `native_on` returns null handles, and the other operations do nothing. `close_on` is idempotent. A window that the system destroys with its owner returns one `CloseRequested` from `next_on`, then fails with `Unsupported`, and releasing it is harmless. `detach` takes the window out of the `Window`'s care, so the caller closes it with `close_on`, and `adopt` gives a handle an owner again; `Options.owner` is a `WindowHandle`, and `Window` does not appear in an optional. Opening with an owner fails with `Unsupported` when the owner has closed or belongs to another `Display`.

The names avoid the private `connect` in `std/ui/platform/window_linux.low:123`, because a module has one function namespace. `platform.Options` gains `owner: WindowHandle?`, `min_width`, `min_height`, `placement: platform.Placement`, `taskbar` and `activate`. `platform.Placement` repeats the three variants of `ui.Placement`; its default is `System`, so `platform.open` places windows as before. `activate` defaults to `true`, which keeps the single-window behaviour: `ShowWindow` with `SW_SHOW`. With `false` the window is shown with `SW_SHOWNOACTIVATE`, whether on open or by `show`, and the library never calls `SetForegroundWindow` or `SetFocus`. The Win32 fixtures open every window with `activate: false` and drive it only by posting messages to its handle, never with `SendInput`, `SendKeys`, `keybd_event`, `SetCursorPos` or `SetForegroundWindow`, so they take no focus from the person running them. `platform.open(o)` stays: it opens a `Display`, opens one window in it, and gives the window ownership of that display, so `Window`'s drop closes both as it does today (`std/ui/platform/platform.low:262-265`).

- **Win32.** `DisplayState` holds the `lock_thread` call, the class registrations, `SetProcessDpiAwarenessContext`, `OleInitialize`, the list of the display's windows, and a message-only window (`HWND_MESSAGE`) that wakers post `WM_APP` to. The message-only window has its own class, `LowDisplay`, and procedure, because `LowWindow`'s procedure reads a `*WindowState` from `GWLP_USERDATA`. `LowDisplay` is registered when the display opens, and `LowWindow` before each window's `CreateWindowExW`, so a display whose windows were all released, or that has none yet, never depends on another display's registration. The thread is unlocked once, when the display closes. `platform.open` makes one display per call, so `win32_pair` runs two displays on one thread, and a program can hold several displays. Low has no module-level variables to count them, so each display's close attempts `UnregisterClassW` for both classes and ignores the failure while a window of the class exists: `UnregisterClassW` fails only then, and an open display with no window does not count, which is why `LowWindow` is registered again at the next window and not once for the display.
- **Wayland.** `DisplayState` holds the `wl_display`, registry, globals, seat, pointer, keyboard, xkb state, key repeat, data device, text input and cursor-shape device, and the single pump green thread (`std/ui/platform/window_linux.low:21-82`). A `Window` holds its `wl_surface`, `xdg_surface`, `xdg_toplevel`, size, scale, configure state and event queue.

### Message pump and wait

`ui.run` makes one call to `platform.wait(display, timeout)`, then `platform.next(w, 0)` for each window.

- **Win32.** `wait` runs `pump` once, which fills every window's queue. It returns if any queue is non-empty. Otherwise it waits in `MsgWaitForMultipleObjectsEx` (`std/ui/platform/window_windows.low:169-179`), which already wakes for input to any window on the thread. `next(w, 0)` then only takes `w`'s queue. `pending_key` stays on each window, because it indexes that window's queue and `WM_CHAR` pairs with the key press of the window that received it; the pump (`:194`) clears it for every window of the display.
- **Wayland.** `wait` dispatches the shared connection, then selects on the pump's ready channel, the display waker and the timeout, the next key repeat or the idle limit, as `wait_input` does for one window (`std/ui/platform/window_linux.low:254-290`).
- The loop computes the timeout as the minimum over windows of the next timer, the next reveal deadline, and zero when any window is due (`std/ui/ui.low:125-126`, `std/ui/clock.low:52-64`).

### Waker and `post`

- `platform.waker(w)` keeps its contract: it posts `WM_APP` to `w`, and `next(w)` returns one `Woken` per `wake` (`docs/design.md:2648`, `compiler/tests/ui/testdata/win32_wake/main.low`).
- `waker_of(d)` is a second waker, owned by the display. On Win32 it posts `WM_APP` to the message-only window, whose procedure records the wake on the display, not on any window queue. On Wayland it sends on the display's channel. A wake ends `wait` even when no window queue filled, and it gives no `Woken` event to any window.
- `Shared.wake` uses the display waker. The loop drains the posts once per iteration (`std/ui/context.low:325-334`). A drained post advances `Shared.generation`, because `drain` today dirties only the state that drains it (`std/ui/context.low:332`). `drain` is also called from `frame` (`:342`) and on `Woken` (`:903`).
- Waking stops depending on any window's life. Today `wake_window` does nothing once its window's `hwnd` is null (`std/ui/platform/window_windows.low:218-222`), so a waker tied to a closed tool window would lose its wakes.
- `post` sends on the shared `posts` channel (`std/ui/context.low:110`, `152`, `320-323`). Its capacity of 1024 and its blocking send stay the same.

### Background tasks

`background_spawn` spawns a green thread, which posts its result through the shared handle (`std/ui/task.low:14-19`, `37-46`). Tasks therefore do not need a window. A `done` callback that touches a closed window's context touches a headless context, which is safe.

### Loop order

Each iteration of `run`:

1. `platform.wait(display, timeout)`.
2. For each open window, in opening order: `next(w, 0)`, `dispatch` every event into that window's context, and handle `CloseRequested` as described in section 1.
3. Drain the shared posts once.
4. Fire every window's due timers.
5. For each window that is due and has an area: build its frame, present it and reveal it.
6. Apply each window's caret and cursor.
7. Close the windows that asked to close, owned windows first.

Opening needs no step of its own, because `open_window` is synchronous.

## 3. GPU

`gpu.State` splits into a shared device and a presenter per window:

| Shared `Device`, one per application | Per-window `Presenter` |
|---|---|
| `VkInstance` with `VK_KHR_surface` and the platform's surface extension (`std/ui/gpu/swapchain.low:25`) | `VkSurfaceKHR` |
| physical device, `VkDevice`, queue, family (`std/ui/gpu/gpu.low:73-88`) | `Swapchain`: images, views and `finished` semaphores (`std/ui/gpu/swapchain.low:3-12`) |
| command pool, set layout, pipeline layout, sampler, descriptor pool (`std/ui/gpu/gpu.low:89-93`) | |
| pipelines keyed by format (`std/ui/gpu/gpu.low:23`, `172-182`) | |
| glyph atlas `Image` and `atlas_ready` (`std/ui/gpu/gpu.low:24-25`, `std/ui/gpu/atlas.low:8-39`) | |
| frame ring: command buffers, fences, `acquired` semaphores, instance buffers, staging (`std/ui/gpu/gpu.low:3-11`, `94-98`) | |

```low
pub fn open_device(first: Surface) -> Device ! Error
pub fn presenter(d: Device, s: Surface) -> Presenter ! Error
pub fn present_to(p: Presenter, s: scene.Scene, mut atlas: scene.Atlas) ! Error
```

`open_for` and `present` keep their signatures. They become a `Gpu` that holds one `Device` and one `Presenter`.

Decisions:

- **One device for every window.** The device is chosen against the first surface (`std/ui/gpu/device.low:72-89`). `presenter` checks each later surface with `vkGetPhysicalDeviceSurfaceSupportKHR` on the chosen family (`std/ui/gpu/device.low:91-95`) and fails with `gpu.Error.NoDevice` when the family cannot present to it, since no device the application holds qualifies for that surface. A device that cannot present to every monitor's window is rare. Section 7 lists it as a risk, not a design driver.
- **The frame ring stays shared.** Presents run one after another on one thread. A frame slot holds a command buffer, a fence and an `acquired` semaphore, none of which depend on a swapchain. `wait_frame` (`std/ui/gpu/gpu.low:184-193`) makes a slot safe to reuse whichever window used it last. The `finished` semaphores belong to each swapchain image and move with the `Presenter`. `frames_in_flight` (`std/ui/gpu/gpu.low:38`) may need to rise to 3 if profiling shows the CPU waiting on fences with three windows due at once.
- **One atlas texture.** `ensure_atlas` rewrites the atlas binding of every frame's descriptor sets (`std/ui/gpu/atlas.low:22-26`). Because the ring is shared, that keeps covering every window. The atlas band upload (`std/ui/gpu/swapchain.low:47-55`) runs in whichever window presents first after a change. The upload is recorded into that frame's command buffer, and every later submit on the one queue sees the new rows.
- **Presenter teardown.** A `Presenter` must be released before its platform window (`std/ui/gpu/swapchain.low:21`). The `Device` keeps its live presenters and targets, so releasing it first destroys their swapchains, surfaces and images, and a later `Presenter` or `Target` release does nothing. Ring frames that presented to it may still be in flight, so closing a window waits on the fence of every submitted ring frame (`wait_frame`), then destroys the swapchain and the `VkSurfaceKHR`, and then releases the platform window. Closing is rare, so waiting on the ring costs little. `vkDeviceWaitIdle` would do the same job with a coarser wait.
- **Pipelines by format.** Two windows whose swapchains choose the same surface format share pipelines. A different format adds an entry (`std/ui/gpu/gpu.low:172-182`).

### Text and images

- One `uitext.System` serves the whole application and moves into `Shared`. Fonts added with `add_font`, pictures added with `add_image` (`std/ui/context.low:171-178`) and the system font set (`std/ui/ui.low:56`) are loaded once.
- One atlas works across windows at different DPI. Built-in glyph tiles are keyed by the font, the glyph and the device pixel size (`std/ui/text/text.low:810-816`). System-font tiles also carry the scale and subpixel variant (`std/ui/text/text.low:848-852`). A glyph drawn at 1.0 and at 1.5 therefore occupies two tiles and is never shared wrongly.
- `end_frame` runs at the start of every window's frame (`std/ui/context.low:341`). It clears the layout cache, which is safe because the cache is valid within one frame. When the atlas ran out of room, `end_frame` also clears the atlas (`std/ui/text/text.low:233-244`). After such a clear, every other window is marked due, so each one packs its glyphs again before it presents. Pixels that a window has already presented are unaffected.
- `params` (`std/ui/text/text.low:213-226`) and subpixel support (`std/ui/gpu/gpu.low:52-54`) come from the one device and the one text system, so every window blends text the same way.

## 4. Context

`State` (`std/ui/context.low:50-114`) splits into two structs:

| Per window: `State` | Shared: `Shared` |
|---|---|
| `background`, `dirty`, `closed`, `revealed`, `rendered_generation` | `text` (fonts, atlas, images) |
| `width`, `height`, `scale`, `rem`, `viewport` | `keymap` |
| `hitboxes`, `hits`, `virtuals`, `inside`, `wheel`, `rects`, `last_rects`, `deferred`, `anchor_shifts`, `anchors_painted`, `painted`, `concealing`, `root_hitbox` | `generation`, which signals advance |
| `layers`, `layer_draws`, `tip` | `posts`, `wake` |
| `pointer`, `pointer_inside`, `hovered`, `pressed`, `held`, `buttons`, `modifiers`, `stopped` | `clock`, `manual_ms` |
| `focus`, `pending_focus`, `scroll`, `window_focused` | `copy`, `paste`, `memory` |
| `fields`, `areas`, `drag`, `area_drag`, `dragged`, `drag_view`, `press_point` | `quit` |
| `caret`, `applied_caret`, `applied_cursor` | `windows`: the list of per-window states, for marking all due and firing all timers |
| `maximized`, `minimize`, `zoom` | `open`: the closure `ui.open` installs to make a platform window and presenter. `headless` installs one that makes a headless context. |
| `pending_keys`, `pending_timer`, `timers`, `next_timer` | `rendering`: `true` while a frame is being built, so `open_window` and `close_window` refuse to run inside `render` |
| `window`: the window's own `Window` handle | `main`: the first window, which `main_window` returns |

- `State` gains `shared: *Shared` and `window`. `Context` stays `{ state: *State }`, so every function that takes a `Context` keeps its signature.
- Timers stay per window. Today they serve the chord timeout and tooltips, both of which belong to a window (`std/ui/clock.low:17-21`), and the clock they read is shared.
- `rem` stays per window. A tool window copies the opener's `rem` when it opens, and `set_rem_size` on any context sets it for that window only. Section 7 lists this as an open question.
- Low has no module-level variables (AGENTS.md, "Language notes"), so the window list is the `windows` field on `Shared`, reached through any context.

## 5. Behaviour per backend

### Win32

- **Class registration.** Each window registers `LowWindow` before `CreateWindowExW`, and the `Display` registers `LowDisplay` when it opens. A class that exists already gives `ERROR_CLASS_ALREADY_EXISTS`, which `register_class` accepts. Unregistering follows the threading section: it is attempted at each display's close, and the attempt fails harmlessly while windows of the class remain. Releasing one display therefore never leaves another display unable to open a window, whichever order the displays and `platform.open` windows are released in.
- **Window procedure.** Each window keeps its `*WindowState` behind `GWLP_USERDATA`, set in `WM_NCCREATE` (`std/ui/platform/messages_windows.low:6-31`). Its click counter, capture, caret, IME, cursor, drop target and hit test are already per window (`std/ui/platform/window_windows.low:7-40`).
- **Owner.** `CreateWindowExW` passes the owner's `hwnd` as `hWndParent` with `WS_OVERLAPPEDWINDOW`. It passes `null` when there is no owner. That makes an owned top-level window, not a child, which stays above its owner and minimises with it.
- **Taskbar.** An owned window has no taskbar button unless it has `WS_EX_APPWINDOW`, so `WS_EX_APPWINDOW` is set only when the window is owned and `taskbar` is `true`; an unowned window already has a button and gets no extended style. `taskbar: false` on an unowned window sets `WS_EX_TOOLWINDOW`, which also gives a smaller caption and keeps the window out of Alt+Tab. The option's documentation says so.
- **Title.** `set_title` calls `SetWindowTextW` with the title converted by `wide` (`std/ui/platform/win32_windows.low:156-162`).
- **Size and centring.** `fit_window` sizes the window from its DPI (`std/ui/platform/window_windows.low:110-131`) and passes `SWP_NOMOVE`. With `CentreOnOwner` or `CentreOnScreen` it also computes a position: the owner's `GetWindowRect`, or the monitor from `MonitorFromWindow`, clamped to that monitor's `GetMonitorInfoW` work area. It drops `SWP_NOMOVE` for that call. `platform` has no main window of its own: "the main window" is the first window of the display that is still open, so a window with no owner is centred on it, `CentreOnScreen` takes its monitor, and the first window of a display is centred on the primary monitor. The size that is centred is the size after `min_width` and `min_height` are applied. A placed window is created at its final rectangle, computed with the DPI that `GetDpiForMonitor` reads for the target monitor, so it starts on that monitor and `GetDpiForWindow` reads the same DPI. `GetWindowRect`, `MonitorFromWindow` and `GetMonitorInfoW` join `std/ui/platform/win32_windows.low:1-58`.
- **Minimum size.** The window procedure handles `WM_GETMINMAXINFO`. With a minimum it sets `ptMinTrackSize` to the minimum content size times the window's current scale, plus the frame from `AdjustWindowRectExForDpi`, or plus the insets from `frame_insets` when `appears_transparent` is set (`std/ui/platform/titlebar_windows.low:47-59`). A DPI change recomputes the minimum on the next `WM_GETMINMAXINFO`. With no minimum it passes the message to `DefWindowProcW`.
- **DPI per monitor.** Each window reads its own DPI (`std/ui/platform/window_windows.low:111`) and handles its own `WM_DPICHANGED` (`std/ui/platform/messages_windows.low:193-203`). A tool window on a 150 percent monitor beside a main window at 100 percent gets `Resized(…, 1.5)` and paints at 1.5. Only `SetProcessDpiAwarenessContext` moves to the `Display`.
- **Cloak until first present (#205).** Each window opens with `hidden: true`, cloaked through `DWMWA_CLOAK` before `ShowWindow` (`std/ui/platform/cloak_windows.low:13-24`). `run` uncloaks each window after its own first successful `present` (`std/ui/ui.low:101-107`, `std/ui/platform/cloak_windows.low:26-42`). `revealed` is already per window. The 500 ms deadline (`std/ui/ui.low:47`, `109-116`) becomes per window too. It is measured from the later of the window's opening and the start of `run`. That keeps the contract of `std/ui/ui.low:120` and the zero-area timing of `win32_appreveal` for the main window, and gives a tool window opened mid-run its own 500 ms.
- **Failed present.** A failed present on a tool window closes that window and passes the error to its `closed` callback as `Failed`. A failed present on the main window ends `run`, as today (`std/ui/ui.low:136`).
- **Close order.** Destroying an owner destroys its owned windows first. The owned window's `WM_NCDESTROY` clears `GWLP_USERDATA` (`std/ui/platform/messages_windows.low:16-19`). Its Low `close_window` would then call `DestroyWindow` on a dead handle, get `destroyed = false` and never unpin its anchor (`std/ui/platform/window_windows.low:231-241`). To prevent this, `ui` closes owned windows before their owner, and the window procedure clears `s.hwnd` on `WM_NCDESTROY`, so a window that the system destroys is released cleanly. `WM_DESTROY` calls `RevokeDragDrop` and unpins the drop target, so OLE never references unpinned memory, and queues one `CloseRequested` unless the window was closed from Low, which `next_on` returns once.
- **Activation.** `activate` calls `ShowWindow(SW_RESTORE)` when the window is minimised (`IsIconic`), then `SetForegroundWindow`. Keyboard focus follows through `WM_SETFOCUS` and `WM_KILLFOCUS` (`std/ui/platform/messages_windows.low:79-87`).

### Wayland

- **One connection.** Every window of a `Display` is a surface on its connection. Seat-level listeners take the display as user data. Surface-level listeners (`preferred_buffer_scale`, `configure` and `close`, `std/ui/platform/window_linux.low:155-164`) keep the window as user data.
- **Routing input.** `wl_pointer.enter` and `wl_keyboard.enter` carry the surface. The display looks the surface up in its window list and records it as the pointer focus or the keyboard focus. Motion, button, axis, key and modifier events go to the window that has that focus (`std/ui/platform/input_linux.low:188-290`). Text input already compares the surface on `enter` (`std/ui/platform/text_input_linux.low:16-18`) and moves from window to display. Key repeat follows the keyboard-focused window.
- **Owner.** `xdg_toplevel.set_parent` with the owner's toplevel (`std/ui/platform/wayland_linux.low:1974`). This requires a shared connection: a toplevel from another connection cannot be a parent.
- **Minimum size.** `xdg_toplevel.set_min_size` (`std/ui/platform/wayland_linux.low:2009`), in logical pixels, committed before the first configure.
- **Limits.** A Wayland client cannot place its toplevel, so `Placement` has no effect. Compositors commonly centre a parented toplevel over its parent. A client also cannot control a taskbar entry, so `taskbar` has no effect. Both limits are documented on the options, as `TitlebarOptions` documents its own (`std/ui/platform/platform.low:17`).
- **Decorations.** A Wayland window has no decorations (`std/ui/platform/platform.low:17`), so a tool window there has no system title bar or close button. The application must draw them, as the main window does. Issue #37 tracks `xdg-decoration` and client-side decorations, which would give tool windows a frame for free.
- **Reveal.** A surface is mapped at its first buffer commit (`std/ui/platform/platform.low:11`), so a tool window is invisible until its first present without a cloak.
- **Clipboard.** The selection belongs to the seat, so `write_clipboard` and `read_clipboard` move to the display. Every window shares them, as the shared `copy` and `paste` closures do.

### Headless test contexts

- Section 1 describes `open_window` on a `headless` context. No platform or GPU is involved, so `std/ui/*_test.low` can drive two windows on any host.
- `compiler/tests/ui` covers the platform and GPU paths, with fixtures in the style of `win32_pair` and `win32_appreveal` (`compiler/tests/ui/win32_tests.low:154-361`). They use the `win32support` helpers `ready` and `post` (`compiler/tests/ui/testdata/win32support`). Wayland fixtures run only where a compositor is present (`compiler/tests/ui/window_tests.low:72-109`).

## 6. Migration and order

### The single-window API is unchanged

- `ui.open` connects a `Display`, opens the main window in it, opens a `Device` and its `Presenter`, and creates `Shared` and the main window's `State`. `App` keeps its private fields and its public `cx`.
- `ui.run(app, render)` runs the multi-window loop. With one window, each step does what the current loop does, in the same order. The `ui_test`, `reveal_test` and `compiler/tests/ui` fixtures pass unchanged and serve as the check.
- `platform.open`, `platform.next`, `platform.waker` and `gpu.open_for` with `gpu.present` keep their signatures and behaviour. They are thin wrappers over `Display`, and over `Device` with `Presenter`.
- `signal`, `get`, `set`, `update`, `background_spawn`, `spawn_into`, `post`, `handle`, `headless` and `advance_clock` keep their signatures. With one window, `Shared` has one entry, and "every window" means that one.

### Pull requests

Each pull request builds, passes `low test std/ui`, `low test std/ui/platform`, `low test std/ui/gpu` and `low run compiler/tests/ui`, and can be reviewed on its own.

1. **Split the context into `Shared` and per-window `State`** (`std/ui`). Signals point at `Shared` and advance `generation`. Posts, the waker, the clock, the clipboard, the keymap and the text system move to `Shared`. `headless` creates its own `Shared`. `open_window` exists for headless contexts only, and `ui.open` installs no opener yet. Tests in `std/ui/window_test.low`:
   - a `set` from window B makes window A due;
   - two `headless` calls stay isolated;
   - `advance_clock` fires timers in both windows;
   - a chord pending in A does not complete in B;
   - `bind_keys` in A binds in B;
   - `spawn_into` on a signal created in a closed window still delivers;
   - `close_window` on a headless window runs `closed(Requested)` and leaves the context usable;
   - `open_window` from inside `render` fails with `Platform(Unsupported(…))`;
   - a post drained by window A makes window B due.
2. **Split the GPU into `Device` and `Presenter`** (`std/ui/gpu`). `open_for` and `present` become wrappers. Tests:
   - existing render fixtures pass unchanged;
   - `std/ui/gpu/device_test.low` makes one `Device` and two `Presenter`s on two windows: each presenter has its own swapchain at its own window's size, presents alternate between the windows, an atlas uploaded through either window is sampled correctly by the other (offscreen readback on a `Target` of the same `Device`), a `Target` from another device is rejected with `WrongDevice`, a `Device` released before its presenters and targets leaves their release harmless, and a draw that fails after the image is acquired leaves the frame slot reusable;
   - a new `compiler/tests/ui` fixture `win32_twopresent` opens two platform windows, makes one `Device` and two `Presenter`s, presents different scenes to each, and checks each window's own pixels with `PrintWindow` after each window uploads the atlas, after forty alternating presents and after the first presenter is released;
   - `wayland_twopresent` runs the same presents where a compositor runs, and checks that they succeed, since a Wayland window's pixels cannot be read back.
3. **Add the Win32 `Display` and `wait`** (`std/ui/platform`, Windows). The lock, class, DPI awareness, OLE and the message-only waker move into it. `platform.open` wraps it. `WM_NCDESTROY` clears `hwnd`. Tests:
   - `win32_pair` passes unchanged;
   - a new `win32_wait` fixture: a message posted to the second window ends `wait` on the display, one wait takes all the wakes that arrived, a wake from another green thread ends a blocked `wait`, a wake still ends `wait` after a window has closed and with none open, `wait` runs to its timeout less one clock tick, and key events for two windows of a display keep their text apart, while a `WM_CHAR` that arrives in a later pump than its key press is dropped;
   - `win32_classonce`: two windows close in either order, the window class stays registered while the display lives, two displays open at once can each open windows after the other is released, and a `platform.open` window released while a display is held leaves the display able to open windows;
   - `win32_handles`: a `DisplayHandle` held by a closure opens a window and ends a `wait_on` through `waker_of_on`, and after the display is released `open_in_on` and `wait_on` fail with `Unsupported` and a wake does nothing.
4. **Add the Win32 window options**: owner, taskbar, min size, placement and `set_title`. Tests in a `win32_owned` fixture:
   - `GetWindow(GW_OWNER)` returns the owner;
   - `WS_EX_APPWINDOW` is set for `taskbar: true`;
   - `SendMessageW(WM_GETMINMAXINFO)` reports the minimum at 100 and 150 percent;
   - a centred window's rectangle centre matches its owner's within a pixel, and the window stays inside the work area;
   - the owner's styles: an unowned window with a taskbar button has neither `WS_EX_APPWINDOW` nor `WS_EX_TOOLWINDOW`, `taskbar: false` clears `WS_EX_APPWINDOW` on an owned window and sets `WS_EX_TOOLWINDOW` on an unowned one;
   - a window smaller than its minimum is centred at the minimum within a pixel;
   - an owner from another `Display` fails with `Unsupported`;
   - when the system destroys an owned window together with its owner, `next` returns one `CloseRequested` and then fails with `Unsupported`, the owned window's release reports no `DestroyWindow` failure and unpins its anchor, and its drop target's pins are free before the release. `std/ui` exposes no pin count, so the fixture fills the 65,536-entry pin table (`pin_capacity`) to a known free count, releases the owner and pins exactly the number of entries the destroyed window gave back, which panics when they were not unpinned. The check depends on `pin_capacity`;
   - 40 owned windows, destroyed by `DestroyWindow` on the owner or by releasing the owner `Window` first, leave their pins free.
   - a `win32_windowhandles` fixture: a closure holds a `WindowHandle` and sets the title and waits; `detach`, `close_on` and `adopt` are idempotent and leave no `DestroyWindow` failure; a stale handle fails with `Unsupported`; a window closed by `close_on` and then released closes its display once.
   - `win32_open` asserts that a window opened with `activate: false` is neither the foreground nor the active window.
5. **Add the Wayland shared connection** (`std/ui/platform`, Linux). Split `WindowState` into display and window state, route input by surface, and add `set_parent` and `set_min_size`. Tests:
   - existing `wayland_*` fixtures pass;
   - `wayland_pair`: two windows on one connection, each configured, with keyboard `enter` routed by surface;
   - `wayland_parent`: a window opened with an owner sends `set_parent`, checked against `WAYLAND_DEBUG=1` output.
6. **Add `ui.open_window` with platform windows, and the multi-window `run`** (`std/ui`). This includes per-window reveal deadlines, per-window failure, close ordering and the `closed` callbacks. It also introduces copyable `DeviceHandle` and `PresenterHandle` views of the GPU types with an idempotent close; the `_on` forms of `target`, `render` and `read` take those views, and `supports_subpixel` gains a `Device` form. Tests:
   - `ui_test` and `reveal_test` pass unchanged;
   - a `win32_apptool` fixture: a click in the main window opens a tool window, which is cloaked after `open_window` and uncloaked after its first frame;
   - a signal set in the tool window changes the main window's next frame;
   - `WM_CLOSE` to the tool window runs `closed(Requested)` and the main window keeps running;
   - `WM_CLOSE` to the main window ends `run` after closing the tool window first.
7. **Document and add an example**: `docs/design.md` `std/ui` and `std/ui/platform` sections, and `std/ui/examples/tool_window.low`, which `lowtest examples` builds.

Pull requests 3 and 4 do not depend on 1 and 2. Pull request 5 depends on nothing in `std/ui` either. Pull request 6 needs 1 to 4, and needs 5 for Linux.

## 7. Risks and open questions

- **Every window renders on every `set`.** A dashboard with a heavy grid renders again when a settings toggle changes. The follow-up is read tracking: `get` records the reading window's bit on the cell while `frame` runs (the current window is known from `Shared`), and `set` makes due only the windows that read the cell. It changes no signature, so it can come after pull request 6.
- **Sequential FIFO presents.** Swapchains use `VK_PRESENT_MODE_FIFO_KHR` with `minImageCount + 1` images (`std/ui/gpu/swapchain.low:121`, `136`). Acquire normally returns at once. But two windows due in one iteration on monitors with different refresh rates can block on acquire for one vblank each. This needs measuring with three windows animating, before deciding between per-window acquire timeouts and `MAILBOX` where it is supported.
- **The Win32 modal size and move loop.** While the user drags any window's frame, `DispatchMessageW` stays inside `DefWindowProcW`'s modal loop, and `run` does not iterate, so no window repaints until the drag ends. This affects one window today too. Rendering from `WM_SIZE` or a `WM_TIMER` inside the modal loop is a separate change.
- **A device that cannot present to a later surface.** With several adapters, a window on a monitor driven by another GPU may not be presentable from the chosen queue family. The design fails `open_window` with `Gpu(NoDevice)`. A presenter on a second device would need a second atlas and a second frame ring.
- **Shared atlas pressure.** Three windows at different scales hold three sets of tiles in one 1024-square atlas (`std/ui/text/text.low:8`). A clear in one window's frame makes every window pack again. Growing the atlas, or making one atlas per distinct scale, can follow if clears show up in profiles.
- **Drag between windows.** `on_drag` state is per window (`std/ui/context.low:89-93`), so dragging a tab from the main window into a tool window is not supported. Supporting it needs pointer capture across windows and a drag owned by `Shared`.
- **Popups outside the window.** Menus and popovers are clamped to their window's viewport (`std/ui/layer.low:279-296`, `341-343`). Popups that reach past the window edge would be popup windows (`WS_POPUP`, or `xdg_popup` on Wayland), which is a separate design.
- **`rem` per window or shared?** A theme that sets the rem once probably wants it shared. Per window is the conservative choice, because it keeps `set_rem_size` local.
- **Window position and size persistence.** Restoring a tool window where the user left it needs `position(w)` and a `Placement.At(x, y)` variant. Wayland cannot honour it.
- **Re-entrancy of `open_window`.** Calling it from a handler while `dispatch` iterates the event list of another window is safe, because the loop iterates a copy of the window list taken before dispatch. That copy needs a test in pull request 6.
- **Posting to a full channel from the UI thread** blocks forever (`std/ui/context.low:152`, `320-323`). This exists today, and more windows posting make it more likely. A non-blocking `post` that grows the queue is out of scope here.
