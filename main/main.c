#include "bsp_battery.h"
#include "bsp_display.h"
#include "bsp_i2c.h"
#include "esp_log.h"
#include "esp_pm.h"
#include "usb_standby.h"
#include "lvgl.h"
#include "nvs_flash.h"
#include "vokie_ble.h"
#include "ui_status.h"

static const char *TAG = "main";

void app_main(void)
{
    ESP_LOGI(TAG, "YiYan firmware starting");
#if CONFIG_PM_ENABLE
    const esp_pm_config_t power = {
        .max_freq_mhz = 160,
        .min_freq_mhz = 40,
        .light_sleep_enable = true,
    };
    ESP_ERROR_CHECK(esp_pm_configure(&power));
#endif
    // Hold the USB console awake before any idle task can reach light sleep.
    usb_standby_init();
    esp_err_t err = nvs_flash_init();
    if (err == ESP_ERR_NVS_NO_FREE_PAGES || err == ESP_ERR_NVS_NEW_VERSION_FOUND) {
        ESP_ERROR_CHECK(nvs_flash_erase());
        err = nvs_flash_init();
    }
    ESP_ERROR_CHECK(err);

    if (bsp_i2c_init() != ESP_OK) {
        ESP_LOGW(TAG, "I2C initialization failed; audio may be unavailable");
    }
    // Initialize before UI/audio workers start using the shared bus. A missing
    // gauge is optional hardware; the status UI will display an unknown SOC.
    if (bsp_battery_init() != ESP_OK) {
        ESP_LOGW(TAG, "Battery unavailable; displaying --%%");
    }
    if (bsp_display_init() == ESP_OK && bsp_lvgl_init()) {
        bsp_display_backlight(65);
        ui_status_init();
    } else {
        ESP_LOGW(TAG, "Display initialization failed; continuing headless");
    }
    // BLE starts advertising before a host connects. Keep the UI explicit
    // about that waiting state; READY is reserved for a completed host
    // handshake.
    ui_status_set_state(UI_STATUS_DISCONNECTED, "Waiting for connection");
    err = vokie_ble_start();
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "YiYan BLE startup failed: %s", esp_err_to_name(err));
    } else {
        ESP_LOGI(TAG, "Ready: advertise as Yi Yan; click UP to talk");
    }
}
