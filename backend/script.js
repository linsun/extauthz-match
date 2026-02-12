// Valentine's Day themed - make additional real requests to show matches
const resources = [
    '/protected/api/love-letter.json',
    '/protected/api/heart.json',
    '/protected/css/valentine.css',
    '/protected/js/cupid.js',
    '/protected/img/rose.png'
];

let heartCount = 0;
let xCount = 0;

// Make actual requests and track their responses
function makeRequest(resource, index) {
    const matchItem = document.querySelectorAll('.match-item')[index];
    const matchResult = document.querySelector(`[data-index="${index}"]`);
    
    fetch(resource)
        .then(response => {
            // If response is OK (200-299), show heart
            if (response.status != 403) {
                matchResult.textContent = '❤️';
                matchItem.classList.remove('pending');
                matchItem.classList.add('matched');
                heartCount++;
                document.getElementById('heartCount').textContent = heartCount;
            } else {
                // If response is error (403, 404, etc), show X
                matchResult.textContent = '❌';
                matchItem.classList.remove('pending');
                matchItem.classList.add('rejected');
                xCount++;
                document.getElementById('xCount').textContent = xCount;
            }
        })
        .catch(() => {
            // Network error or denied - show X
            matchResult.textContent = '❌';
            matchItem.classList.remove('pending');
            matchItem.classList.add('rejected');
            xCount++;
            document.getElementById('xCount').textContent = xCount;
        });
}

// Make requests after a short delay to show multiple auth requests
window.addEventListener('DOMContentLoaded', () => {
    setTimeout(() => {
        resources.forEach((resource, index) => {
            setTimeout(() => {
                makeRequest(resource, index);
            }, index * 800);
        });
    }, 1000);
});
