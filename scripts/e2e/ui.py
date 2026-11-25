#!/usr/bin/env python3
"""Drive the mergediff window through AT-SPI and XTest (end-to-end test helper).

Usage: ui.py STEP [STEP ...]
  wait                 wait until the mergediff window's buttons are accessible
  dump                 print buttons, chips and checkboxes
  click NAME[#N]       click the N-th (0-based) button, chip or checkbox named NAME
  key COMBO            e.g. ctrl+z, ctrl+s, shift+F7, Escape (XTest)
  type TEXT            type TEXT into the focused widget
  scroll X Y N         move the pointer to screen X,Y and wheel N notches (N<0 = up)
  clickat X Y          left-click at screen X,Y (XTest), e.g. to focus a pane
  pixel X Y            print the colour at screen X,Y as #rrggbb
  find TEXT            wait until some element's text contains TEXT (exit 1 if not)
  samples TITLE SECS   wait for a window titled TITLE, then print the brightness
                       (0-255) of its toolbar every frame for SECS seconds
  sleep SECONDS
"""
import sys
import time

import pyatspi

KEYSYMS = {
    "ctrl": 0xFFE3,
    "shift": 0xFFE1,
    "z": 0x7A,
    "s": 0x73,
    "a": 0x61,
    "Escape": 0xFF1B,
    "F5": 0xFFC2,
    "F7": 0xFFC4,
    "Return": 0xFF0D,
    "End": 0xFF57,
}


def find_app():
    desktop = pyatspi.Registry.getDesktop(0)
    for app in desktop:
        if app is not None and app.name and "mergediff" in app.name.lower():
            return app
    return None


# Clickable controls: buttons, and the directory view's filter chips (radio
# buttons) and checkboxes.
CONTROL_ROLES = ("push button", "button", "radio button", "toggle button", "check box")


def buttons(app):
    return pyatspi.findAllDescendants(app, lambda n: n.getRoleName() in CONTROL_ROLES)


def wait(timeout=30):
    end = time.time() + timeout
    while time.time() < end:
        app = find_app()
        if app is not None and any(True for _ in buttons(app)):
            return app
        time.sleep(0.3)
    sys.exit("mergediff window not found")


def click(app, spec, timeout=10):
    # The UI may still be settling (e.g. strip buttons are drawn on the next
    # animation frame, later on a cold first start), so wait for the button
    # to exist and be enabled instead of failing at once.
    name, _, nth = spec.partition("#")
    n = int(nth or 0)
    end = time.time() + timeout
    while True:
        matches = [b for b in buttons(app) if b.name == name]
        if n < len(matches) and matches[n].getState().contains(pyatspi.STATE_SENSITIVE):
            break
        if time.time() > end:
            sys.exit(f"button {name!r}#{n} not found or disabled; have {[b.name for b in buttons(app)]}")
        time.sleep(0.2)
    matches[n].queryAction().doAction(0)
    time.sleep(0.4)


def xdisplay():
    from Xlib import display

    return display.Display()


def active_title(d):
    """Title of the window that has the input focus ("" if unknown)."""
    root = d.screen().root
    active = root.get_full_property(d.intern_atom("_NET_ACTIVE_WINDOW"), 0)
    if not active or not active.value or not active.value[0]:
        return ""
    w = d.create_resource_object("window", active.value[0])
    name = w.get_full_property(d.intern_atom("_NET_WM_NAME"), d.intern_atom("UTF8_STRING"))
    if name and name.value:
        return name.value.decode() if isinstance(name.value, bytes) else str(name.value)
    return w.get_wm_name() or ""


def require_focus(d):
    """Refuses to send input unless a mergediff window is active, so keys
    and clicks can never land in another application (e.g. a terminal)."""
    title = active_title(d)
    if not title.startswith("mergediff —"):
        sys.exit(f"refusing to send input: the active window is {title!r}, not mergediff")


def key(combo):
    from Xlib import X
    from Xlib.ext import xtest

    d = xdisplay()
    require_focus(d)
    codes = [d.keysym_to_keycode(KEYSYMS[p]) for p in combo.split("+")]
    for c in codes:
        xtest.fake_input(d, X.KeyPress, c)
    for c in reversed(codes):
        xtest.fake_input(d, X.KeyRelease, c)
    d.sync()
    time.sleep(0.4)


def scroll(x, y, n):
    from Xlib import X
    from Xlib.ext import xtest

    d = xdisplay()
    require_focus(d)
    xtest.fake_input(d, X.MotionNotify, x=x, y=y)
    d.sync()
    time.sleep(0.1)
    button = 5 if n > 0 else 4
    for _ in range(abs(n)):
        xtest.fake_input(d, X.ButtonPress, button)
        xtest.fake_input(d, X.ButtonRelease, button)
        d.sync()
        time.sleep(0.03)
    time.sleep(0.5)


def clickat(x, y):
    from Xlib import X
    from Xlib.ext import xtest

    d = xdisplay()
    require_focus(d)
    xtest.fake_input(d, X.MotionNotify, x=x, y=y)
    d.sync()
    time.sleep(0.1)
    xtest.fake_input(d, X.ButtonPress, 1)
    xtest.fake_input(d, X.ButtonRelease, 1)
    d.sync()
    time.sleep(0.4)


def pixel(x, y):
    from Xlib import X

    d = xdisplay()
    raw = d.screen().root.get_image(x, y, 1, 1, X.ZPixmap, 0xFFFFFFFF).data
    b, g, r = raw[0], raw[1], raw[2]  # 32-bit TrueColor: BGRX
    print(f"#{r:02x}{g:02x}{b:02x}")


def find_window(d, title, timeout=30):
    """The X window whose title starts with title, polled until it exists."""
    root = d.screen().root
    clients = d.intern_atom("_NET_CLIENT_LIST")
    name_atom, utf8 = d.intern_atom("_NET_WM_NAME"), d.intern_atom("UTF8_STRING")
    end = time.time() + timeout
    while time.time() < end:
        prop = root.get_full_property(clients, 0)
        for wid in prop.value if prop else []:
            w = d.create_resource_object("window", wid)
            try:
                name = w.get_full_property(name_atom, utf8)
            except Exception:
                continue
            value = name.value.decode() if name and isinstance(name.value, bytes) else ""
            if value.startswith(title):
                return w
        time.sleep(0.005)
    sys.exit(f"window {title!r} not found")


def samples(title, seconds):
    """Brightness of the window's toolbar from the moment the window exists:
    catches a frame painted in the wrong theme at startup."""
    from Xlib import X

    d = xdisplay()
    w = find_window(d, title)
    end = time.time() + seconds
    while time.time() < end:
        try:
            geo = w.get_geometry()
            raw = w.get_image(geo.width // 2, 20, 1, 1, X.ZPixmap, 0xFFFFFFFF).data
            print((raw[0] + raw[1] + raw[2]) // 3, flush=True)
        except Exception:
            pass  # not viewable yet
        time.sleep(0.01)


def texts(node, depth=0):
    """Yields the text of node and its descendants (accessible name or Text)."""
    if node is None or depth > 40:
        return
    if node.name:
        yield node.name
    try:
        t = node.queryText()
        yield t.getText(0, -1)
    except NotImplementedError:
        pass
    for child in node:
        yield from texts(child, depth + 1)


def find(app, text, timeout=10):
    end = time.time() + timeout
    while time.time() < end:
        if any(text in t for t in texts(app)):
            return
        time.sleep(0.3)
    sys.exit(f"text {text!r} not found")


def type_text(text):
    require_focus(xdisplay())
    pyatspi.Registry.generateKeyboardEvent(0, text, pyatspi.KEY_STRING)
    time.sleep(0.4)


def dump(app):
    for b in buttons(app):
        state = "enabled" if b.getState().contains(pyatspi.STATE_SENSITIVE) else "disabled"
        print(f"button {b.name!r} ({state})")


def main(argv):
    app = None
    i = 0
    while i < len(argv):
        step = argv[i]
        if step == "wait":
            app = wait()
        elif step == "dump":
            dump(app or wait())
        elif step == "click":
            i += 1
            click(app or wait(), argv[i])
        elif step == "key":
            i += 1
            key(argv[i])
        elif step == "type":
            i += 1
            type_text(argv[i])
        elif step == "scroll":
            scroll(int(argv[i + 1]), int(argv[i + 2]), int(argv[i + 3]))
            i += 3
        elif step == "clickat":
            clickat(int(argv[i + 1]), int(argv[i + 2]))
            i += 2
        elif step == "pixel":
            pixel(int(argv[i + 1]), int(argv[i + 2]))
            i += 2
        elif step == "samples":
            samples(argv[i + 1], float(argv[i + 2]))
            i += 2
        elif step == "find":
            i += 1
            find(app or wait(), argv[i])
        elif step == "sleep":
            i += 1
            time.sleep(float(argv[i]))
        else:
            sys.exit(f"unknown step {step}")
        i += 1


if __name__ == "__main__":
    main(sys.argv[1:])
