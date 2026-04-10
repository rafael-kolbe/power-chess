/**
 * Builds a power card DOM node using static template PNGs under /public/cards/.
 * All templates share dimensions 639×965; text is overlaid with percentage positioning.
 *
 * @typedef {"continuous"|"counter"|"power"|"retribution"} PowerCardType
 * @typedef {Object} PowerCardOptions
 * @property {PowerCardType|string} type Card frame / color family
 * @property {string} [name] Title shown in top parchment strip
 * @property {string} [description] Main rules text
 * @property {string} [example] Shown in footer; also used as tooltip on the "i" hint
 * @property {number|string} [mana] Mana cost (top-right blue orb)
 * @property {number|string} [ignition] Ignition value (bottom-left)
 * @property {number|string} [cooldown] Cooldown / recharge (bottom-right)
 * @property {string} [cardWidth] CSS length for --card-width (default 220px)
 * @property {boolean} [exampleOnHoverOnly] If true, example is hidden until hover (class power-card--example-hover)
 */

(function () {
  const TYPE_META = {
    continuous: { className: "power-card--continuous" },
    counter: { className: "power-card--counter" },
    power: { className: "power-card--power" },
    retribution: { className: "power-card--retribution" }
  };

  /**
   * Normalizes type string to a known PowerCardType.
   * @param {string} raw
   * @returns {keyof typeof TYPE_META}
   */
  function normalizeType(raw) {
    const k = String(raw || "continuous").toLowerCase();
    return TYPE_META[k] ? k : "continuous";
  }

  /**
   * Creates a card element from options.
   * @param {PowerCardOptions} opts
   * @returns {HTMLElement}
   */
  function createPowerCard(opts) {
    const type = normalizeType(opts.type);
    const meta = TYPE_META[type];
    const article = document.createElement("article");
    article.className = `power-card ${meta.className}`;
    if (opts.exampleOnHoverOnly) {
      article.classList.add("power-card--example-hover");
    }
    if (opts.cardWidth) {
      article.style.setProperty("--card-width", opts.cardWidth);
    }

    const name = opts.name != null ? String(opts.name) : "";
    const desc = opts.description != null ? String(opts.description) : "";
    const example = opts.example != null ? String(opts.example) : "";
    const mana = opts.mana != null && opts.mana !== "" ? String(opts.mana) : "—";
    const ignition = opts.ignition != null && opts.ignition !== "" ? String(opts.ignition) : "—";
    const cooldown = opts.cooldown != null && opts.cooldown !== "" ? String(opts.cooldown) : "—";

    article.setAttribute("aria-label", name || "Card");

    const titleEl = document.createElement("h3");
    titleEl.className = "power-card__title";
    titleEl.textContent = name;

    const manaEl = document.createElement("span");
    manaEl.className = "power-card__mana";
    manaEl.setAttribute("aria-label", `Mana ${mana}`);
    manaEl.textContent = mana;

    const body = document.createElement("div");
    body.className = "power-card__body";

    const descEl = document.createElement("p");
    descEl.className = "power-card__desc";
    descEl.textContent = desc;

    body.appendChild(descEl);

    if (example) {
      const footer = document.createElement("footer");
      footer.className = "power-card__footer";

      const info = document.createElement("span");
      info.className = "power-card__info";
      info.setAttribute("role", "button");
      info.setAttribute("title", `Example: ${example}`);
      info.setAttribute("tabindex", "0");
      info.textContent = "i";

      const exampleEl = document.createElement("p");
      exampleEl.className = "power-card__example";
      exampleEl.textContent = `Example: ${example}`;

      footer.appendChild(info);
      footer.appendChild(exampleEl);
      body.appendChild(footer);
    }

    const ignEl = document.createElement("span");
    ignEl.className = "power-card__ignition";
    ignEl.setAttribute("aria-label", `Ignition ${ignition}`);
    ignEl.textContent = ignition;

    const cdEl = document.createElement("span");
    cdEl.className = "power-card__cooldown";
    cdEl.setAttribute("aria-label", `Cooldown ${cooldown}`);
    cdEl.textContent = cooldown;

    article.appendChild(titleEl);
    article.appendChild(manaEl);
    article.appendChild(body);
    article.appendChild(ignEl);
    article.appendChild(cdEl);

    return article;
  }

  function readLocaleForCards() {
    try {
      return localStorage.getItem("powerChessLocale") || "en-US";
    } catch (_) {
      return "en-US";
    }
  }

  /** @param {HTMLElement} root */
  function mountPreviewIfPresent(root) {
    if (!root) return;
    const getCat = globalThis.getLocalizedCardCatalog;
    if (typeof getCat !== "function") {
      return;
    }
    const all = getCat(readLocaleForCards());
    const pick = [];
    for (const t of ["continuous", "counter", "power", "retribution"]) {
      const c = all.find((x) => x.type === t);
      if (c) pick.push(c);
    }
    root.replaceChildren();
    for (const c of pick) {
      root.appendChild(
        createPowerCard({
          type: c.type,
          name: c.name,
          description: c.description,
          example: c.example,
          mana: c.mana,
          ignition: c.ignition,
          cooldown: c.cooldown,
          cardWidth: "200px"
        })
      );
    }
  }

  globalThis.createPowerCard = createPowerCard;

  function schedulePreview() {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", () => mountPreviewIfPresent(document.getElementById("cardPreviewRow")));
    } else {
      mountPreviewIfPresent(document.getElementById("cardPreviewRow"));
    }
  }

  document.addEventListener("powerchess:locale", () => {
    mountPreviewIfPresent(document.getElementById("cardPreviewRow"));
  });

  schedulePreview();
})();
