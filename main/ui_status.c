#include "ui_status.h"

#include "bsp_battery.h"
#include "bsp_button.h"
#include "bsp_display.h"
#include "bsp_pins.h"
#include "button_scan.h"
#include "esp_err.h"
#include "esp_timer.h"
#include "esp_log.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "lvgl.h"
#include "ui_battery.h"
#include "ui_idle.h"
#include "go_voice_logo_asset.h"
#include <stdio.h>
#include <string.h>

#define UI_BG 0x0B1018
#define UI_PANEL 0x151D28
#define UI_WHITE 0xF6F8FC
#define UI_INK 0x202A38
#define UI_MUTED 0x7F8B9E
#define UI_BATTERY_MUTED 0x646C78
#define UI_BLUE 0x67D8FF
#define UI_GREEN 0x78E0A4
#define UI_ORANGE 0xFFB454
#define UI_RED 0xFF7180
#define UI_CONTENT_X 0
#define UI_CONTENT_W BSP_LCD_W
#define UI_HEADER_X 0
#define UI_HEADER_W BSP_LCD_W
#define UI_HEADER_Y 245
#define UI_BATTERY_W 66
#define UI_BATTERY_RIGHT_MARGIN 16
#define UI_BATTERY_X (BSP_LCD_W - UI_BATTERY_RIGHT_MARGIN - UI_BATTERY_W)
#define UI_BATTERY_Y 21
#define UI_LOGO_SIZE 64
#define UI_LOGO_X ((BSP_LCD_W - UI_LOGO_SIZE) / 2)
// Keep the 64 px brand asset in the existing logo slot.
#define UI_LOGO_Y 132
// "伊言" is a pre-rendered bitmap: the enabled Montserrat fonts have no CJK glyphs.
#define UI_CAPTION_W 56
#define UI_CAPTION_H 28
#define UI_CAPTION_X ((BSP_LCD_W - UI_CAPTION_W) / 2)
#define UI_CAPTION_Y 243
#define UI_STATE_Y 53
#define UI_DETAIL_Y 86
#define UI_HINT_FADE_IN_MS 160
#define UI_HINT_FADE_OUT_MS 220
#define UI_HINT_GROUP_X 178
#define UI_HINT_GROUP_Y 126
#define UI_HINT_GROUP_W 42
#define UI_HINT_ROW_H 22
#define UI_BATTERY_POLL_US (30LL * 1000LL * 1000LL)

static lv_obj_t *s_screen;
static lv_obj_t *s_state;
static lv_obj_t *s_detail;
static lv_obj_t *s_button_hints;
static lv_obj_t *s_hint_panel;
static lv_obj_t *s_hint_lines[3];
static lv_obj_t *s_hint_labels[3];
static lv_obj_t *s_battery;
static portMUX_TYPE s_state_lock = portMUX_INITIALIZER_UNLOCKED;
static ui_idle_t s_idle;
static uint8_t s_backlight_level;
static bool s_hints_visible;
static ui_status_hint_t s_displayed_hints;
static TaskHandle_t s_task;

// LVGL retains these arrays. Startup keeps all three original guide paths;
// a single hint starts at the SEND row and points to its physical key.
static const lv_point_precise_t s_hint_all_lines[3][4] = {
    {{220, 137}, {225, 137}, {232, 57}, {238, 57}},
    {{220, 159}, {225, 159}, {232, 159}, {238, 159}},
    {{220, 181}, {225, 181}, {232, 261}, {238, 261}},
};
static const lv_point_precise_t s_hint_single_lines[3][4] = {
    {{220, 159}, {225, 159}, {232, 57}, {238, 57}},
    {{220, 159}, {225, 159}, {232, 159}, {238, 159}},
    {{220, 159}, {225, 159}, {232, 261}, {238, 261}},
};

static lv_obj_t *box(lv_obj_t *parent, int x, int y, int w, int h, uint32_t color,
                     int radius)
{
    lv_obj_t *obj = lv_obj_create(parent);
    lv_obj_remove_flag(obj, LV_OBJ_FLAG_SCROLLABLE);
    lv_obj_set_pos(obj, x, y);
    lv_obj_set_size(obj, w, h);
    lv_obj_set_style_bg_color(obj, lv_color_hex(color), 0);
    lv_obj_set_style_bg_opa(obj, LV_OPA_COVER, 0);
    lv_obj_set_style_border_width(obj, 0, 0);
    lv_obj_set_style_radius(obj, radius, 0);
    lv_obj_set_style_pad_all(obj, 0, 0);
    return obj;
}

static lv_obj_t *label(lv_obj_t *parent, const char *text, const lv_font_t *font,
                       uint32_t color, int width)
{
    lv_obj_t *obj = lv_label_create(parent);
    lv_label_set_text(obj, text);
    lv_obj_set_style_text_font(obj, font, 0);
    lv_obj_set_style_text_color(obj, lv_color_hex(color), 0);
    if (width > 0) {
        lv_obj_set_width(obj, width);
        lv_obj_set_style_text_align(obj, LV_TEXT_ALIGN_CENTER, 0);
    }
    return obj;
}

static void set_backlight(uint8_t percent)
{
    if (percent == s_backlight_level) return;
    s_backlight_level = percent;
    bsp_display_backlight(percent);
}

static const char *state_name(ui_status_state_t state)
{
    switch (state) {
    case UI_STATUS_RECORDING: return "LISTENING";
    case UI_STATUS_PROCESSING: return "THINKING";
    case UI_STATUS_SUCCESS: return "SENT";
    case UI_STATUS_ERROR: return "ERROR";
    case UI_STATUS_DISCONNECTED: return "OFFLINE";
    case UI_STATUS_STARTING: return "STARTING";
    default: return "READY";
    }
}

static uint32_t state_color(ui_status_state_t state)
{
    switch (state) {
    case UI_STATUS_RECORDING: return UI_ORANGE;
    case UI_STATUS_PROCESSING: return UI_BLUE;
    case UI_STATUS_SUCCESS: return UI_GREEN;
    case UI_STATUS_ERROR: return UI_RED;
    case UI_STATUS_DISCONNECTED: return UI_MUTED;
    default: return UI_BLUE;
    }
}

static const lv_image_dsc_t s_go_voice_logo = {
    .header = {
        .magic = LV_IMAGE_HEADER_MAGIC,
        .cf = LV_COLOR_FORMAT_NATIVE,
        .w = UI_LOGO_SIZE,
        .h = UI_LOGO_SIZE,
        .stride = UI_LOGO_SIZE * 2,
    },
    .data_size = sizeof(s_go_voice_logo_data),
    .data = s_go_voice_logo_data,
};

static const lv_image_dsc_t s_yiyan_caption = {
    .header = {
        .magic = LV_IMAGE_HEADER_MAGIC,
        .cf = LV_COLOR_FORMAT_NATIVE,
        .w = UI_CAPTION_W,
        .h = UI_CAPTION_H,
        .stride = UI_CAPTION_W * 2,
    },
    .data_size = sizeof(s_yiyan_caption_data),
    .data = s_yiyan_caption_data,
};

static void draw_go_voice(void)
{
    // Brand mark artwork, composited onto UI_BG as RGB565.
    lv_obj_t *mark = lv_image_create(s_screen);
    lv_image_set_src(mark, &s_go_voice_logo);
    lv_obj_set_pos(mark, UI_LOGO_X, UI_LOGO_Y);
}

static void draw_button_rail(void)
{
    // Button hints and leader lines appear after a physical-key action.
    const uint32_t colors[3] = {UI_ORANGE, UI_BLUE, UI_GREEN};
    const char *actions[3] = {"VOICE", "SEND", "UNDO"};
    s_button_hints = lv_obj_create(s_screen);
    lv_obj_remove_style_all(s_button_hints);
    lv_obj_set_pos(s_button_hints, 0, 0);
    lv_obj_set_size(s_button_hints, BSP_LCD_W, BSP_LCD_H);
    lv_obj_remove_flag(s_button_hints, LV_OBJ_FLAG_SCROLLABLE);
    for (int i = 0; i < 3; ++i) {
        lv_obj_t *line = lv_line_create(s_button_hints);
        s_hint_lines[i] = line;
        lv_obj_remove_style_all(line);
        lv_obj_set_pos(line, 0, 0);
        lv_obj_set_size(line, BSP_LCD_W, BSP_LCD_H);
        lv_line_set_points(line, s_hint_all_lines[i], 4);
        lv_obj_set_style_line_width(line, 1, 0);
        lv_obj_set_style_line_color(line, lv_color_hex(colors[i]), 0);
        lv_obj_set_style_line_opa(line, LV_OPA_50, 0);
        lv_obj_set_style_line_rounded(line, true, 0);
    }
    s_hint_panel = box(s_button_hints, UI_HINT_GROUP_X, UI_HINT_GROUP_Y,
                       UI_HINT_GROUP_W, UI_HINT_ROW_H * 3, UI_PANEL, 7);
    for (int i = 0; i < 3; ++i) {
        lv_obj_t *text = label(s_button_hints, actions[i], &lv_font_montserrat_10,
                               colors[i], UI_HINT_GROUP_W);
        s_hint_labels[i] = text;
        lv_obj_set_pos(text, UI_HINT_GROUP_X, UI_HINT_GROUP_Y +
            i * UI_HINT_ROW_H + (UI_HINT_ROW_H - lv_font_montserrat_10.line_height) / 2);
    }
    lv_obj_add_flag(s_button_hints, LV_OBJ_FLAG_HIDDEN);
}

static void draw_battery(void)
{
    // Keep battery status visible whenever the screen is lit, independently
    // of button hint fades. Battery updates do not wake the backlight.
    s_battery = label(s_screen, LV_SYMBOL_BATTERY_EMPTY " --%",
                      &lv_font_montserrat_12, UI_BATTERY_MUTED, UI_BATTERY_W);
    lv_obj_set_style_text_align(s_battery, LV_TEXT_ALIGN_RIGHT, 0);
    lv_label_set_long_mode(s_battery, LV_LABEL_LONG_CLIP);
    lv_obj_set_pos(s_battery, UI_BATTERY_X, UI_BATTERY_Y);
}

static void set_button_hints_visible(ui_status_hint_t hints)
{
    if (!s_button_hints) return;
    const bool visible = hints != UI_STATUS_HINT_NONE;
    if (visible && hints != s_displayed_hints) {
        const bool all = hints == UI_STATUS_HINT_ALL;
        for (int i = 0; i < 3; ++i) {
            if (hints & (1 << i)) {
                lv_obj_set_y(s_hint_labels[i], UI_HINT_GROUP_Y +
                    (all ? i : 1) * UI_HINT_ROW_H +
                    (UI_HINT_ROW_H - lv_font_montserrat_10.line_height) / 2);
                lv_line_set_points(s_hint_lines[i],
                    all ? s_hint_all_lines[i] : s_hint_single_lines[i], 4);
                lv_obj_remove_flag(s_hint_labels[i], LV_OBJ_FLAG_HIDDEN);
                lv_obj_remove_flag(s_hint_lines[i], LV_OBJ_FLAG_HIDDEN);
            } else {
                lv_obj_add_flag(s_hint_labels[i], LV_OBJ_FLAG_HIDDEN);
                lv_obj_add_flag(s_hint_lines[i], LV_OBJ_FLAG_HIDDEN);
            }
        }
        lv_obj_set_y(s_hint_panel, UI_HINT_GROUP_Y + (all ? 0 : UI_HINT_ROW_H));
        lv_obj_set_height(s_hint_panel, (all ? 3 : 1) * UI_HINT_ROW_H);
        s_displayed_hints = hints;
    }
    if (visible == s_hints_visible) return;
    s_hints_visible = visible;
    lv_anim_delete(s_button_hints, NULL);
    if (visible) {
        lv_obj_remove_flag(s_button_hints, LV_OBJ_FLAG_HIDDEN);
        lv_obj_move_foreground(s_button_hints);
        lv_obj_set_style_opa(s_button_hints, LV_OPA_TRANSP, 0);
        lv_obj_fade_in(s_button_hints, UI_HINT_FADE_IN_MS, 0);
    } else {
        lv_obj_fade_out(s_button_hints, UI_HINT_FADE_OUT_MS, 0);
    }
}

// Called only by initialization / the status worker with the LVGL lock held.
static void render_state(const ui_idle_t *state, ui_status_hint_t hints)
{
    const uint32_t color = state_color(state->state);
    lv_label_set_text(s_state, state_name(state->state));
    lv_obj_set_style_text_color(s_state, lv_color_hex(color), 0);
    lv_label_set_text(s_detail, state->message[0] ? state->message : "Ready");
    set_button_hints_visible(hints);
    lv_obj_set_style_opa(s_screen, LV_OPA_COVER, 0);
}

static ui_idle_t snapshot(void)
{
    taskENTER_CRITICAL(&s_state_lock);
    const ui_idle_t state = s_idle;
    taskEXIT_CRITICAL(&s_state_lock);
    return state;
}

static void status_task(void *arg)
{
    (void)arg;
    int64_t next_battery_us = 0;
    int battery_soc = -1, battery_mv = -1;
    bool battery_dirty = true;
    uint32_t rendered_revision = 0;
    ui_status_hint_t rendered_hints = UI_STATUS_HINT_NONE;
    for (;;) {
        const ui_idle_t state = snapshot();
        const int64_t now_us = esp_timer_get_time();
        const uint8_t brightness = ui_idle_brightness(&state, now_us);
        const ui_status_hint_t hints = ui_idle_hints(&state, now_us);
        bool retry = false;
        if (brightness) {
            // SLPOUT waits 120 ms. Do not hold the LVGL lock across it.
            if (bsp_display_set_sleep(false) != ESP_OK) {
                retry = true;
            } else if (bsp_lvgl_lock(100)) {
                set_backlight(brightness);
                ESP_ERROR_CHECK(bsp_lvgl_set_paused(false));
                if (state.revision != rendered_revision || hints != rendered_hints) {
                    render_state(&state, hints);
                    rendered_revision = state.revision;
                    rendered_hints = hints;
                }
                bsp_lvgl_unlock();
            } else {
                retry = true;
            }
        } else if (bsp_lvgl_lock(100)) {
            ESP_ERROR_CHECK(bsp_lvgl_set_paused(true));
            set_backlight(0);
            bsp_lvgl_unlock();
            if (bsp_display_set_sleep(true) != ESP_OK) retry = true;
        } else {
            retry = true;
        }
        const bool idle_scan = button_scan_idle(brightness == 0, ui_idle_active(&state));
        const esp_err_t scan_err = bsp_button_set_idle_scan(idle_scan);
        if (scan_err != ESP_OK && scan_err != ESP_ERR_INVALID_STATE) retry = true;

        if (now_us >= next_battery_us) {
            // I2C stays outside the LVGL lock. Sampling while off does not
            // invalidate objects, restart animations or resume the LVGL tick.
            battery_soc = bsp_battery_soc();
            battery_mv = bsp_battery_mv();
            battery_dirty = true;
            next_battery_us = esp_timer_get_time() + UI_BATTERY_POLL_US;
        }
        if (brightness && battery_dirty) {
            if (bsp_lvgl_lock(100)) {
                static const char *const icons[] = {
                    LV_SYMBOL_BATTERY_EMPTY, LV_SYMBOL_BATTERY_1,
                    LV_SYMBOL_BATTERY_2, LV_SYMBOL_BATTERY_3, LV_SYMBOL_BATTERY_FULL,
                };
                const ui_battery_view_t view = ui_battery_view(battery_soc, battery_mv);
                lv_label_set_text_fmt(s_battery, "%s %s", icons[view.bars], view.text);
                lv_obj_set_style_text_color(s_battery,
                    lv_color_hex(view.low ? UI_RED : UI_BATTERY_MUTED), 0);
                battery_dirty = false;
                bsp_lvgl_unlock();
            } else {
                retry = true;
            }
        }
        const int64_t after_work_us = esp_timer_get_time();
        int64_t delay_us = ui_idle_wait_us(&state, after_work_us, next_battery_us);
        // A blocking battery read may cross a dim/off/hint deadline.
        if (ui_idle_brightness(&state, after_work_us) != brightness ||
            ui_idle_hints(&state, after_work_us) != hints) delay_us = 0;
        if (retry && delay_us > 20000) delay_us = 20000;
        TickType_t ticks = pdMS_TO_TICKS((delay_us + 999) / 1000);
        // Pending notifications wake immediately, including during I2C work.
        ulTaskNotifyTake(pdTRUE, ticks ? ticks : 1);
    }
}

void ui_status_init(void)
{
    if (s_screen || !bsp_lvgl_lock(1000)) return;
    const int64_t now = esp_timer_get_time();
    s_idle = (ui_idle_t){
        .state = UI_STATUS_STARTING, .hints = UI_STATUS_HINT_ALL,
        .last_touch_us = now, .hints_deadline_us = now + UI_HINT_TIMEOUT_US,
        .revision = 1,
    };
    s_screen = lv_obj_create(NULL);
    lv_obj_remove_style_all(s_screen);
    lv_obj_set_style_bg_color(s_screen, lv_color_hex(UI_BG), 0);
    lv_obj_set_style_bg_opa(s_screen, LV_OPA_COVER, 0);

    // Center the brand caption below the logo and the upper status group.
    lv_obj_t *caption = lv_image_create(s_screen);
    lv_image_set_src(caption, &s_yiyan_caption);
    lv_obj_set_pos(caption, UI_CAPTION_X, UI_CAPTION_Y);
    draw_go_voice();
    draw_button_rail();
    draw_battery();

    s_state = label(s_screen, "", &lv_font_montserrat_20, UI_BLUE, UI_CONTENT_W);
    lv_obj_set_pos(s_state, UI_CONTENT_X, UI_STATE_Y);
    s_detail = lv_label_create(s_screen);
    lv_obj_set_style_text_font(s_detail, &lv_font_montserrat_14, 0);
    lv_obj_set_style_text_color(s_detail, lv_color_hex(UI_MUTED), 0);
    lv_obj_set_style_text_align(s_detail, LV_TEXT_ALIGN_CENTER, 0);
    lv_obj_set_width(s_detail, UI_CONTENT_W);
    lv_label_set_long_mode(s_detail, LV_LABEL_LONG_DOT);
    lv_obj_set_pos(s_detail, UI_CONTENT_X, UI_DETAIL_Y);

    lv_screen_load(s_screen);
    render_state(&s_idle, UI_STATUS_HINT_ALL);
    set_backlight(ui_idle_brightness(&s_idle, now));
    bsp_lvgl_unlock();
    // Battery I2C and diagnostic/error logging need additional stack headroom.
    if (xTaskCreate(status_task, "ui_status", 3072, NULL, 2, &s_task) != pdPASS) {
        ESP_LOGE("ui_status", "Failed to start status worker");
    }
}

void ui_status_touch(ui_status_hint_t hint)
{
    if (hint != UI_STATUS_HINT_VOICE && hint != UI_STATUS_HINT_SEND &&
        hint != UI_STATUS_HINT_UNDO && hint != UI_STATUS_HINT_ALL) return;
    taskENTER_CRITICAL(&s_state_lock);
    ui_idle_touch(&s_idle, hint, esp_timer_get_time());
    taskEXIT_CRITICAL(&s_state_lock);
    if (s_task) xTaskNotifyGive(s_task);
}

void ui_status_set_state(ui_status_state_t state, const char *message)
{
    taskENTER_CRITICAL(&s_state_lock);
    const bool changed = ui_idle_set_state(&s_idle, state, message, esp_timer_get_time());
    taskEXIT_CRITICAL(&s_state_lock);
    if (changed && s_task) xTaskNotifyGive(s_task);
}
