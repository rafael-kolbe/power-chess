/**
 * Builds a power card DOM node using static template PNGs under /public/cards/.
 * All templates share dimensions 639×965; text is overlaid with percentage positioning.
 *
 * @typedef {"continuous"|"counter"|"power"|"retribution"} PowerCardType
 * @typedef {Object} PowerCardOptions
 * @property {PowerCardType|string} type Card frame / color family
 * @property {string} [name] Title shown in top parchment strip
 * @property {string} [description] Main rules text
 * @property {string} [example] Optional; toggled vs description via parchment footer button
 * @property {number|string} [mana] Mana cost (top-right blue orb)
 * @property {number|string} [ignition] Ignition value (bottom-left)
 * @property {number|string} [cooldown] Cooldown / recharge (bottom-right)
 * @property {string} [cardWidth] CSS length for --card-width (default 220px)
 */

(function () {
  /** @type {WeakMap<HTMLElement, ResizeObserver>} */
  const cardLayoutObservers = new WeakMap();

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
   * Frame type label for the green-band stamp (English caps, same for all UI locales).
   * @param {string} rawType
   * @returns {string}
   */
  function getCardTypeLabel(rawType) {
    const t = normalizeType(rawType);
    const labels = {
      power: "POWER CARD",
      counter: "COUNTER CARD",
      retribution: "RETRIBUTION CARD",
      continuous: "CONTINUOUS CARD"
    };
    return labels[t] || labels.power;
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
    const locale = readLocaleForCards();
    if (locale === "pt-BR") {
      article.classList.add("power-card--locale-pt");
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

    const typeLabel = getCardTypeLabel(type);
    article.setAttribute("aria-label", name ? `${name} — ${typeLabel}` : typeLabel);

    const titleEl = document.createElement("h3");
    titleEl.className = "power-card__title";
    const titleClip = document.createElement("span");
    titleClip.className = "power-card__title-clip";
    const titleInner = document.createElement("span");
    titleInner.className = "power-card__title-inner";
    titleInner.textContent = name;
    titleClip.appendChild(titleInner);
    titleEl.appendChild(titleClip);

    const manaEl = document.createElement("span");
    manaEl.className = "power-card__mana";
    manaEl.setAttribute("aria-label", `Mana ${mana}`);
    manaEl.textContent = mana;

    const body = document.createElement("div");
    body.className = "power-card__body";

    const textStack = document.createElement("div");
    textStack.className = "power-card__text-stack";

    const descEl = document.createElement("p");
    descEl.className = "power-card__desc";
    descEl.textContent = desc;
    textStack.appendChild(descEl);

    const typeEl = document.createElement("span");
    typeEl.className = "power-card__type-stamp";
    typeEl.setAttribute("aria-hidden", "true");
    typeEl.textContent = typeLabel;

    if (example) {
      const exampleEl = document.createElement("p");
      exampleEl.className = "power-card__example";
      exampleEl.textContent = example;
      exampleEl.setAttribute("hidden", "");
      textStack.appendChild(exampleEl);

      const loc = locale;
      const exLabel = loc === "pt-BR" ? "Exemplo" : "Example";
      const descLabel = loc === "pt-BR" ? "Descrição" : "Description";

      const footer = document.createElement("footer");
      footer.className = "power-card__footer";

      const toggleBtn = document.createElement("button");
      toggleBtn.type = "button";
      toggleBtn.className = "power-card__toggle";
      toggleBtn.textContent = exLabel;
      toggleBtn.setAttribute("aria-label", loc === "pt-BR" ? "Mostrar texto de exemplo" : "Show example text");
      toggleBtn.setAttribute("aria-pressed", "false");

      toggleBtn.addEventListener("click", () => {
        const on = article.classList.toggle("power-card--show-example");
        if (on) {
          descEl.setAttribute("hidden", "");
          exampleEl.removeAttribute("hidden");
        } else {
          exampleEl.setAttribute("hidden", "");
          descEl.removeAttribute("hidden");
        }
        toggleBtn.textContent = on ? descLabel : exLabel;
        toggleBtn.setAttribute("aria-pressed", on ? "true" : "false");
        toggleBtn.setAttribute(
          "aria-label",
          on
            ? loc === "pt-BR"
              ? "Mostrar descrição da carta"
              : "Show card description"
            : loc === "pt-BR"
              ? "Mostrar texto de exemplo"
              : "Show example text"
        );
        finalizePowerCardLayout(article);
      });

      footer.appendChild(toggleBtn);
      body.appendChild(textStack);
      body.appendChild(footer);
    } else {
      body.appendChild(textStack);
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
    article.appendChild(typeEl);
    article.appendChild(ignEl);
    article.appendChild(cdEl);

    queueMicrotask(() => finalizePowerCardLayout(article));

    return article;
  }

  /**
   * Scales the title horizontally so long names stay on one line (squish), never wrap.
   * @param {HTMLElement} titleEl
   */
  function fitCardTitle(titleEl) {
    const inner = titleEl.querySelector(".power-card__title-inner");
    const clip = titleEl.querySelector(".power-card__title-clip");
    if (!inner) return;
    inner.style.transform = "scaleX(1)";
    const maxW = clip ? clip.clientWidth : titleEl.clientWidth;
    const textW = inner.scrollWidth;
    if (maxW > 0 && textW > maxW) {
      const scale = Math.max(0.26, Math.min(1, maxW / textW));
      inner.style.transform = `scaleX(${scale})`;
    }
  }

  /** @type {readonly string[]} */
  const CARD_SHRINK_CLASSES = ["power-card--shrink-text", "power-card--shrink-tight", "power-card--shrink-min"];

  /**
   * Shrinks description/example font stepwise (up to three tiers) when content would overflow
   * the parchment pane—important for long examples after toggling from description.
   * @param {HTMLElement} article
   */
  function adjustCardTypography(article) {
    article.classList.remove(...CARD_SHRINK_CLASSES);
    const stack = article.querySelector(".power-card__text-stack");
    const desc = article.querySelector(".power-card__desc");
    const ex = article.querySelector(".power-card__example");
    const showingEx = article.classList.contains("power-card--show-example") && ex;
    const pane = showingEx && ex ? ex : desc;
    if (!stack || !pane) return;
    if (stack.clientHeight < 8) return;

    const overflows = () => pane.scrollHeight > pane.clientHeight + 1;

    void article.offsetHeight;
    if (!overflows()) return;

    for (const cls of CARD_SHRINK_CLASSES) {
      article.classList.add(cls);
      void article.offsetHeight;
      if (!overflows()) break;
    }
  }

  /**
   * Fits title scale and body fonts after layout; attaches a single ResizeObserver per card.
   * Safe to call again after cloning nodes (each element gets its own observer).
   * @param {HTMLElement} article
   */
  function finalizePowerCardLayout(article) {
    const titleEl = article.querySelector(".power-card__title");
    const run = () => {
      if (titleEl) fitCardTitle(titleEl);
      adjustCardTypography(article);
    };
    requestAnimationFrame(() => {
      run();
      requestAnimationFrame(run);
    });
    if (cardLayoutObservers.has(article)) return;
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(() => run());
    ro.observe(article);
    cardLayoutObservers.set(article, ro);
  }

  globalThis.finalizePowerCardLayout = finalizePowerCardLayout;

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
