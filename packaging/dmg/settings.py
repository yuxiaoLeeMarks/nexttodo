# The disk image of NextTodo, for dmgbuild (see build.sh): the app, a link
# to Applications to drag it to, and one to Privacy & Security, where
# macOS lets an app that is not notarized open, over a background that
# says so. The places of the icons are those packaging_test.go draws the
# background around.
import os.path

app = defines["app"]
name = os.path.basename(app)

files = [app]
symlinks = {
    "Applications": "/Applications",
    "Privacy & Security": "/System/Library/PreferencePanes/Security.prefPane",
}
icon_locations = {
    name: (170, 165),
    "Applications": (490, 165),
    "Privacy & Security": (560, 368),
}
background = defines["background"]
icon = os.path.join(app, "Contents", "Resources", "AppIcon.icns")

window_rect = ((200, 120), (660, 480))
default_view = "icon-view"
icon_size = 96
text_size = 12
show_status_bar = False
show_tab_view = False
show_toolbar = False
show_pathbar = False
show_sidebar = False
include_icon_view_settings = True

format = "ULMO"  # LZMA, the smallest
filesystem = "HFS+"
