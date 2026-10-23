#[path = "native_transition.rs"]
mod native_transition;

use crate::error::{AppError, AppResult};
use crate::http_client::{resolve_proxy, ResolvedProxy};
use crate::repo;
use crate::state::AppState;
use serde::Serialize;
use std::time::Duration;
use tauri::{AppHandle, Manager, State};
use tauri_plugin_updater::UpdaterExt;

const CHECK_TIMEOUT: Duration = Duration::from_millis(15_000);

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UpdateCheckMetadata {
    pub rid: u32,
    pub current_version: String,
    pub version: String,
    pub date: Option<String>,
    pub body: Option<String>,
    pub raw_json: serde_json::Value,
}

fn apply_updater_proxy(
    builder: tauri_plugin_updater::UpdaterBuilder,
    resolved: &ResolvedProxy,
) -> AppResult<tauri_plugin_updater::UpdaterBuilder> {
    match resolved {
        ResolvedProxy::Disabled => Ok(builder.no_proxy()),
        ResolvedProxy::Url(proxy) => {
            let url = url::Url::parse(proxy)
                .map_err(|e| AppError::new("validation_failed", e.to_string()))?;
            Ok(builder.proxy(url))
        }
        ResolvedProxy::Unset => Ok(builder),
    }
}

#[tauri::command]
pub async fn check_app_update(
    app: AppHandle,
    state: State<'_, AppState>,
) -> AppResult<Option<UpdateCheckMetadata>> {
    let settings = state.db.with_conn(repo::settings::get_settings)?;
    let resolved = resolve_proxy(&settings)?;
    match &resolved {
        ResolvedProxy::Url(_) => {
            tracing::info!(mode = %settings.proxy_mode, "checking updates with configured proxy");
        }
        ResolvedProxy::Disabled => {
            tracing::info!(mode = %settings.proxy_mode, "checking updates without proxy");
        }
        ResolvedProxy::Unset => {
            tracing::info!(mode = %settings.proxy_mode, "checking updates with inherited proxy");
        }
    }

    let mut builder = apply_updater_proxy(app.updater_builder().timeout(CHECK_TIMEOUT), &resolved)?;
    let major = native_transition::os_major().map_err(|e| AppError::new("updater", e))?;
    if native_transition::eligible(env!("CARGO_PKG_VERSION"), std::env::consts::OS, major) {
        let endpoint = url::Url::parse(native_transition::ENDPOINT)
            .map_err(|e| AppError::new("updater", e.to_string()))?;
        builder = builder
            .endpoints(vec![endpoint])
            .map_err(|e| AppError::new("updater", e.to_string()))?;
    }
    let updater = builder
        .build()
        .map_err(|e| AppError::new("updater", e.to_string()))?;
    let update = updater
        .check()
        .await
        .map_err(|e| AppError::new("updater", e.to_string()))?;

    let Some(update) = update else {
        return Ok(None);
    };

    let window = app
        .get_webview_window("main")
        .ok_or_else(|| AppError::new("internal", "main webview missing"))?;
    let current_version = update.current_version.clone();
    let version = update.version.clone();
    let date = update.date.map(|d| d.to_string());
    let body = update.body.clone();
    let raw_json = update.raw_json.clone();
    let rid = window.resources_table().add(update);
    Ok(Some(UpdateCheckMetadata {
        rid,
        current_version,
        version,
        date,
        body,
        raw_json,
    }))
}
