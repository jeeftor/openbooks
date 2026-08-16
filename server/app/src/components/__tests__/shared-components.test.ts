// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, h } from "vue";

import StatusDot from "../shared/StatusDot.vue";
import FormatBadge from "../shared/FormatBadge.vue";
import AgeBanner from "../shared/AgeBanner.vue";
import FilterChip from "../shared/FilterChip.vue";
import EmptyPanel from "../shared/EmptyPanel.vue";
import PanelHeader from "../shared/PanelHeader.vue";

// Minimal icon stand-in for EmptyPanel tests
const MockIcon = defineComponent({
  props: { size: Number },
  setup(props) {
    return () => h("svg", { "data-testid": "mock-icon", "data-size": props.size });
  },
});

// ── StatusDot ──────────────────────────────────────────────────────────────────

describe("StatusDot", () => {
  it("shows a green dot and 'Online' title when online=true", () => {
    const wrapper = mount(StatusDot, { props: { online: true } });
    expect(wrapper.classes()).toContain("bg-green-500");
    expect(wrapper.attributes("title")).toBe("Online");
  });

  it("shows a slate dot and 'Offline' title when online=false", () => {
    const wrapper = mount(StatusDot, { props: { online: false } });
    expect(wrapper.classes()).toContain("bg-slate-300");
    expect(wrapper.attributes("title")).toBe("Offline");
  });

  it("does not have bg-green-500 when offline", () => {
    const wrapper = mount(StatusDot, { props: { online: false } });
    expect(wrapper.classes()).not.toContain("bg-green-500");
  });
});

// ── FormatBadge ───────────────────────────────────────────────────────────────

describe("FormatBadge", () => {
  it("renders the format uppercased when format is provided", () => {
    const wrapper = mount(FormatBadge, { props: { format: "epub" } });
    expect(wrapper.text()).toBe("EPUB");
  });

  it("renders nothing when format is undefined", () => {
    const wrapper = mount(FormatBadge, { props: {} });
    expect(wrapper.find("span").exists()).toBe(false);
  });

  it("renders nothing when format is an empty string", () => {
    const wrapper = mount(FormatBadge, { props: { format: "" } });
    expect(wrapper.find("span").exists()).toBe(false);
  });

  it("renders pdf uppercased", () => {
    const wrapper = mount(FormatBadge, { props: { format: "pdf" } });
    expect(wrapper.text()).toBe("PDF");
  });
});

// ── AgeBanner ─────────────────────────────────────────────────────────────────

describe("AgeBanner", () => {
  it("renders the message prop", () => {
    const wrapper = mount(AgeBanner, { props: { message: "Showing cached results." } });
    expect(wrapper.text()).toContain("Showing cached results.");
  });

  it("uses 'Search again' as the default actionLabel", () => {
    const wrapper = mount(AgeBanner, { props: { message: "old data" } });
    expect(wrapper.find("button").text()).toContain("Search again");
  });

  it("renders a custom actionLabel when provided", () => {
    const wrapper = mount(AgeBanner, {
      props: { message: "Stale data", actionLabel: "Refresh now" },
    });
    expect(wrapper.find("button").text()).toContain("Refresh now");
  });

  it("emits 'refresh' when the button is clicked", async () => {
    const wrapper = mount(AgeBanner, { props: { message: "old" } });
    await wrapper.find("button").trigger("click");
    expect(wrapper.emitted("refresh")).toHaveLength(1);
  });

  it("renders the icon slot", () => {
    const wrapper = mount(AgeBanner, {
      props: { message: "old" },
      slots: { icon: '<svg data-testid="refresh-icon" />' },
    });
    expect(wrapper.find('[data-testid="refresh-icon"]').exists()).toBe(true);
  });
});

// ── FilterChip ────────────────────────────────────────────────────────────────

describe("FilterChip", () => {
  it("applies active classes when active=true", () => {
    const wrapper = mount(FilterChip, {
      props: { active: true },
      slots: { default: "EPUB" },
    });
    expect(wrapper.classes()).toContain("bg-brand-400");
    expect(wrapper.classes()).toContain("text-white");
  });

  it("applies inactive classes when active=false", () => {
    const wrapper = mount(FilterChip, {
      props: { active: false },
      slots: { default: "EPUB" },
    });
    expect(wrapper.classes()).not.toContain("bg-brand-400");
    expect(wrapper.classes()).toContain("border-slate-200");
  });

  it("emits 'toggle' when clicked", async () => {
    const wrapper = mount(FilterChip, {
      props: { active: false },
      slots: { default: "All" },
    });
    await wrapper.trigger("click");
    expect(wrapper.emitted("toggle")).toHaveLength(1);
  });

  it("renders slot content", () => {
    const wrapper = mount(FilterChip, {
      props: { active: false },
      slots: { default: "PDF" },
    });
    expect(wrapper.text()).toBe("PDF");
  });
});

// ── EmptyPanel ────────────────────────────────────────────────────────────────

describe("EmptyPanel", () => {
  it("renders the provided icon component with size=28", () => {
    const wrapper = mount(EmptyPanel, {
      props: { icon: MockIcon },
      slots: { default: "Nothing here yet." },
    });
    const icon = wrapper.find('[data-testid="mock-icon"]');
    expect(icon.exists()).toBe(true);
    expect(icon.attributes("data-size")).toBe("28");
  });

  it("renders the default slot text", () => {
    const wrapper = mount(EmptyPanel, {
      props: { icon: MockIcon },
      slots: { default: "No search history yet." },
    });
    expect(wrapper.text()).toContain("No search history yet.");
  });
});

// ── PanelHeader ───────────────────────────────────────────────────────────────

describe("PanelHeader", () => {
  it("renders the default slot content", () => {
    const wrapper = mount(PanelHeader, {
      slots: { default: "42 searches" },
    });
    expect(wrapper.text()).toContain("42 searches");
  });

  it("renders the actions slot content", () => {
    const wrapper = mount(PanelHeader, {
      slots: {
        default: "entries",
        actions: '<button data-testid="clear-btn">Clear</button>',
      },
    });
    expect(wrapper.find('[data-testid="clear-btn"]').exists()).toBe(true);
    expect(wrapper.find('[data-testid="clear-btn"]').text()).toBe("Clear");
  });

  it("renders both slots simultaneously", () => {
    const wrapper = mount(PanelHeader, {
      slots: {
        default: "10 files",
        actions: "<span>sort</span>",
      },
    });
    expect(wrapper.text()).toContain("10 files");
    expect(wrapper.text()).toContain("sort");
  });
});
