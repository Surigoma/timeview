import i18next from "i18next";

void i18next.init({
  lng: "ja",
  fallbackLng: "ja",
  initAsync: false,
  resources: {
    ja: {
      translation: {
        errors: {
          operationFailed: "操作に失敗しました",
          noAutomaticRetry:
            "自動再送は行いません。現在の状態を確認してください。",
        },
      },
    },
  },
});

export function operationError(message?: string) {
  return `${message || i18next.t("errors.operationFailed")}。${i18next.t("errors.noAutomaticRetry")}`;
}
