"""Writes the marshalled body of each portal call in std/portal/portal_test.low, as hexadecimal, using jeepney as the reference marshaller."""
import os
from jeepney import DBusAddress, new_method_call

here = os.path.dirname(os.path.abspath(__file__))
portal = DBusAddress("/org/freedesktop/portal/desktop", bus_name="org.freedesktop.portal.Desktop")


def body_of(interface, member, signature, body):
    message = new_method_call(portal.with_interface(interface), member, signature, body)
    data = message.serialise(serial=1)
    length = int.from_bytes(data[4:8], "little")
    return data[len(data) - length:]


def write(name, body):
    with open(os.path.join(here, name + ".hex"), "w") as out:
        out.write(body.hex() + "\n")


images = ("Images", [(0, "*.png"), (0, "*.jpg"), (1, "image/gif")])
text = ("Text", [(1, "text/plain")])

write("open_file", body_of(
    "org.freedesktop.portal.FileChooser", "OpenFile", "ssa{sv}",
    ("wayland:abc123", "Open picture", {
        "handle_token": ("s", "low1"),
        "modal": ("b", True),
        "accept_label": ("s", "Choose"),
        "multiple": ("b", True),
        "directory": ("b", False),
        "filters": ("a(sa(us))", [images, text]),
        "current_filter": ("(sa(us))", text),
        "current_folder": ("ay", b"/home/me/pictures\0"),
    })))

write("save_file", body_of(
    "org.freedesktop.portal.FileChooser", "SaveFile", "ssa{sv}",
    ("x11:2a00007", "Save as", {
        "handle_token": ("s", "low2"),
        "modal": ("b", False),
        "filters": ("a(sa(us))", [text]),
        "current_name": ("s", "notes.txt"),
        "current_folder": ("ay", b"/tmp\0"),
        "current_file": ("ay", b"/tmp/notes.txt\0"),
    })))

write("open_uri", body_of(
    "org.freedesktop.portal.OpenURI", "OpenURI", "ssa{sv}",
    ("", "https://example.org/a b", {
        "handle_token": ("s", "low3"),
        "writable": ("b", True),
        "ask": ("b", True),
    })))

write("read_setting", body_of(
    "org.freedesktop.portal.Settings", "Read", "ss",
    ("org.freedesktop.appearance", "color-scheme")))

write("add_notification", body_of(
    "org.freedesktop.portal.Notification", "AddNotification", "sa{sv}",
    ("mail-1", {
        "title": ("s", "New mail"),
        "body": ("s", "Three messages"),
        "icon": ("(sv)", ("themed", ("as", ["mail-unread"]))),
        "priority": ("s", "high"),
        "default-action": ("s", "open"),
        "buttons": ("aa{sv}", [
            {"label": ("s", "Reply"), "action": ("s", "reply")},
            {"label": ("s", "Archive"), "action": ("s", "archive")},
        ]),
    })))
