// Generate some additional requests to auth service
const resources = [
    '/api/config.json',
    '/api/data.json',
    '/css/theme.css',
    '/js/analytics.js',
    '/img/logo.png'
];

// Make requests after a short delay to show multiple auth requests
setTimeout(() => {
    resources.forEach((resource, index) => {
        setTimeout(() => {
            fetch(resource).catch(() => {
                // Expected to fail, just demonstrating multiple requests
            });
        }, index * 500);
    });
}, 1000);
