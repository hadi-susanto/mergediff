#!/usr/bin/env python3
"""Drive the mergediff window through AT-SPI and XTest (end-to-end test helper).

Usage: ui.py STEP [STEP ...]
  wait                 wait until the mergediff window's buttons are accessible
  dump                 print buttons
  click NAME[#N]       click the N-th (0-based) button named NAME
  key COMBO            e.g. ctrl+z, ctrl+s, shift+F7, Escape (XTest)
  type TEXT            type TEXT into the focused widget
  scroll X Y N         move the pointer to screen X,Y and wheel N notches (N<0 = up)
  sleep SECONDS
"""
import sys
import time

import pyatspi

KEYSYMS = {"ctrl": 0xFFE3, "shift": 0xFFE1, "z": 0x7A, "s": 0x73, "a": 0x61, "Escape": 0xFF1B, "F7": 0xFFC4}


def find_app():
    desktop = pyatspi.Registry.getDesktop(0)
    for app in desktop:
        if app is not None and app.name and "mergediff" in app.name.lower():
            return app
    return None


def buttons(app):
    return pyatspi.findAllDescendants(app, lambda n: n.getRoleName() in ("push button", "button"))


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


def key(combo):
    from Xlib import X
    from Xlib.ext import xtest

    d = xdisplay()
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


def type_text(text):
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
        elif step == "sleep":
            i += 1
            time.sleep(float(argv[i]))
        else:
            sys.exit(f"unknown step {step}")
        i += 1


if __name__ == "__main__":
    main(sys.argv[1:])
