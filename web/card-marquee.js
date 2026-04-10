/**
 * Full-width footer carousel: all catalog cards, shuffled on load, infinite horizontal slide.
 */
(function () {
  /**
   * Fisher–Yates shuffle (in-place copy).
   * @template T
   * @param {T[]} arr
   * @returns {T[]}
   */
  function shuffle(arr) {
    const a = arr.slice();
    for (let i = a.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [a[i], a[j]] = [a[j], a[i]];
    }
    return a;
  }

  function currentLocale() {
    try {
      return localStorage.getItem("powerChessLocale") || "en-US";
    } catch (_) {
      return "en-US";
    }
  }

  function buildMarquee() {
    const catalogFn = globalThis.getLocalizedCardCatalog;
    const create = globalThis.createPowerCard;
    const track = document.getElementById("cardMarqueeTrack");
    if (typeof catalogFn !== "function" || typeof create !== "function" || !track) {
      return;
    }

    const catalog = catalogFn(currentLocale());
    if (!catalog.length) {
      return;
    }

    track.replaceChildren();
    const ordered = shuffle(catalog);

    const seq = document.createElement("div");
    seq.className = "card-marquee__sequence";
    seq.setAttribute("aria-hidden", "true");

    const cardWidth = "168px";
    for (const c of ordered) {
      seq.appendChild(
        create({
          type: c.type,
          name: c.name,
          description: c.description,
          example: c.example,
          mana: c.mana,
          ignition: c.ignition,
          cooldown: c.cooldown,
          cardWidth
        })
      );
    }

    const seqB = seq.cloneNode(true);
    seqB.setAttribute("aria-hidden", "true");

    track.appendChild(seq);
    track.appendChild(seqB);

    const durationSec = Math.max(50, ordered.length * 3.2);
    track.style.setProperty("--marquee-duration", `${durationSec}s`);
  }

  function scheduleMarquee() {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", buildMarquee);
    } else {
      buildMarquee();
    }
  }

  document.addEventListener("powerchess:locale", () => {
    buildMarquee();
  });

  scheduleMarquee();
  globalThis.refreshCardMarquee = buildMarquee;
})();
