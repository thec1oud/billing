const express = require('express');
const cors = require('cors');

const app = express();
const PORT = process.env.PORT || 3000;

app.use(cors());
app.use(express.json());

// Proxy requests to the Billing Service
app.get('/api/health', (req, res) => {
    res.json({ status: 'Platform BFF is running!' });
});


app.listen(PORT, () => {
    console.log(`Platform BFF listening on port ${PORT}`);
});
