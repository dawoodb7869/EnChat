#right now this is hosted @ https://enchat-tor-link-server.vercel.app/


from flask import Flask, jsonify
import requests
from bs4 import BeautifulSoup

app = Flask(__name__)

@app.route('/get', methods=['GET'])
def get_download_link():
    url = 'https://www.torproject.org/download/tor/'
    
    try:
        response = requests.get(url)
        response.raise_for_status()
        
        soup = BeautifulSoup(response.content, 'html.parser')
        
        target_text = "Windows (x86_64) "
        link = None
        
        for tag in soup.find_all(string=lambda text: target_text in text):
            parent_td = tag.find_parent('td')
            if parent_td:
                sibling_td = parent_td.find_next_sibling('td')
                if sibling_td:
                    download_link_tag = sibling_td.find('a', class_='downloadLink')
                    if download_link_tag and 'href' in download_link_tag.attrs:
                        link = download_link_tag['href']
                        break
        
        if link:
            return jsonify({"download_link": link})
        else:
            return jsonify({"error": "Matching row not found"}), 404

    except requests.RequestException as e:
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    app.run(debug=True)
