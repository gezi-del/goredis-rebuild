(() => {
  const slides = Array.from(document.querySelectorAll('.slide'));
  const frame = document.getElementById('slide-frame');
  const deck = document.getElementById('deck');
  const viewport = document.getElementById('viewport');
  const previous = document.getElementById('prev');
  const next = document.getElementById('next');
  const fullscreen = document.getElementById('fullscreen');
  const counter = document.getElementById('page-counter');
  const progress = document.getElementById('progress-fill');
  const slideWidth = 1600;
  const slideHeight = 900;
  const initial = Number.parseInt(location.hash.replace('#', ''), 10);
  let current = Number.isInteger(initial) && initial >= 1 && initial <= slides.length ? initial - 1 : 0;

  function fit() {
    const style = getComputedStyle(viewport);
    const availableWidth = viewport.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
    const availableHeight = viewport.clientHeight - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom);
    const scale = Math.max(0.1, Math.min(availableWidth / slideWidth, availableHeight / slideHeight));
    frame.style.width = `${slideWidth * scale}px`;
    frame.style.height = `${slideHeight * scale}px`;
    deck.style.transform = `scale(${scale})`;
  }

  function show(index) {
    const target = Math.max(0, Math.min(slides.length - 1, index));
    slides[current].classList.remove('active');
    slides[current].setAttribute('aria-hidden', 'true');
    current = target;
    slides[current].classList.add('active');
    slides[current].removeAttribute('aria-hidden');
    counter.textContent = `${String(current + 1).padStart(2, '0')} / ${String(slides.length).padStart(2, '0')}`;
    progress.style.width = `${((current + 1) / slides.length) * 100}%`;
    previous.disabled = current === 0;
    next.disabled = current === slides.length - 1;
    history.replaceState(null, '', `#${current + 1}`);
  }

  previous.addEventListener('click', () => show(current - 1));
  next.addEventListener('click', () => show(current + 1));
  fullscreen.addEventListener('click', async () => {
    if (document.fullscreenElement) await document.exitFullscreen();
    else await document.documentElement.requestFullscreen();
  });
  document.addEventListener('keydown', (event) => {
    if (event.key === 'ArrowRight' || event.key === 'PageDown' || event.key === ' ') {
      event.preventDefault();
      show(current + 1);
    } else if (event.key === 'ArrowLeft' || event.key === 'PageUp') {
      event.preventDefault();
      show(current - 1);
    } else if (event.key === 'Home') {
      event.preventDefault();
      show(0);
    } else if (event.key === 'End') {
      event.preventDefault();
      show(slides.length - 1);
    } else if (event.key.toLowerCase() === 'f' && !event.repeat) {
      fullscreen.click();
    }
  });
  window.addEventListener('hashchange', () => {
    const page = Number.parseInt(location.hash.replace('#', ''), 10);
    if (Number.isInteger(page)) show(page - 1);
  });
  window.addEventListener('resize', fit);
  document.addEventListener('fullscreenchange', fit);
  slides.forEach((slide, index) => {
    if (index !== current) slide.setAttribute('aria-hidden', 'true');
  });
  show(current);
  fit();
})();
