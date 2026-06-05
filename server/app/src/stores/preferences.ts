import { defineStore } from "pinia";
import { useLocalStorage } from "@vueuse/core";

export const usePreferencesStore = defineStore("preferences", () => {
  const preferredFormats = useLocalStorage<string[]>("ob-pref-formats", ["epub"]);
  const showUnmatched = useLocalStorage<boolean>("ob-pref-show-unmatched", false);

  function setPreferredFormats(formats: string[]) {
    preferredFormats.value = [...formats];
  }

  function clearPreferredFormats() {
    preferredFormats.value = [];
  }

  return { preferredFormats, showUnmatched, setPreferredFormats, clearPreferredFormats };
});
