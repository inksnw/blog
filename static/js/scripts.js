const height = 300;

makeCollapsible();

function makeCollapsible() {
    const divs = document.querySelectorAll('.highlight');

    divs.forEach((e) => {
        if (e.offsetHeight > height) {
            e.style.maxHeight = height + 'px';
            e.style.overflow = 'hidden';

            const linkWrapper = document.createElement('div');
            const link = document.createElement('a');

            link.href = '';
            link.textContent = '更多';
            link.addEventListener('click', codeToggle);

            linkWrapper.className = 'highlight-link';
            linkWrapper.appendChild(link);
            e.appendChild(linkWrapper);
        }
    });
}

function codeToggle(e) {
    e.preventDefault();

    const link = e.target;
    const linkWrapper = link.parentElement.parentElement;

    if (link.innerHTML === '更多') {
        link.innerHTML = '折叠';
        linkWrapper.style.maxHeight = '';
        linkWrapper.style.overflow = 'none';
        link.parentElement.style.right = '0';
    } else {
        link.innerHTML = '更多';
        linkWrapper.style.maxHeight = height + 'px';
        linkWrapper.style.overflow = 'hidden';
        link.parentElement.style.right = '0.5em';
        scrollToTargetAdjusted(linkWrapper);
    }
}

function scrollToTargetAdjusted(e) {
    const header = document.querySelector('header');
    const headerHeight = window.getComputedStyle(header, null).getPropertyValue('height');
    const headerOffset = Number(headerHeight.replace('px', ''));

    const bodyRect = document.body.getBoundingClientRect().top;
    const elementRect = e.getBoundingClientRect().top;
    const elementPosition = elementRect - bodyRect;

    const offsetPosition = elementPosition - headerOffset;

    window.scrollTo({
        top: offsetPosition, behavior: 'smooth'
    });
}