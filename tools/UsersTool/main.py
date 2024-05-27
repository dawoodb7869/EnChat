import sys
import os
import toml
import hashlib
from pymongo import MongoClient, errors
from PyQt5.QtWidgets import (QApplication, QWidget, QVBoxLayout, QLabel, QMessageBox, 
                             QTableWidget, QTableWidgetItem, QPushButton, QHBoxLayout, 
                             QLineEdit, QDialog, QFormLayout, QSplashScreen)
from PyQt5.QtGui import QIcon, QFont, QColor, QPixmap
from PyQt5.QtCore import Qt, QTimer
import qtawesome as qta
import qdarkstyle

class SplashScreen(QSplashScreen):
    def __init__(self):
        super().__init__(QPixmap(), Qt.WindowStaysOnTopHint)
        self.setWindowFlag(Qt.FramelessWindowHint)
        self.setStyleSheet(qdarkstyle.load_stylesheet())
        self.setFont(QFont('Arial', 24))
        self.showMessage('EnChat User Manager\nLoading...', alignment=Qt.AlignCenter)
        self.resize(600, 400)
        self.center()
    
    def center(self):
        screen = QApplication.primaryScreen()
        screen_geometry = screen.availableGeometry()
        self.move((screen_geometry.width() - self.width()) // 2, (screen_geometry.height() - self.height()) // 2)


class ConfigCheckerApp(QWidget):
    def __init__(self):
        super().__init__()
        self.initUI()
        self.check_config()

    def initUI(self):
        self.setWindowTitle('EnChat User Manager')
        self.setGeometry(100, 100, 900, 600)

        self.layout = QVBoxLayout()
        self.label = QLabel('Checking configuration...', self)
        self.label.setStyleSheet("font-size: 18px;")
        self.layout.addWidget(self.label)

        self.table = QTableWidget()
        self.table.setColumnCount(3)  # Adjust the number of columns as needed
        self.table.setHorizontalHeaderLabels(['Username', 'Password', 'Actions'])
        self.layout.addWidget(self.table)

        self.add_user_button = QPushButton('Add User')
        self.add_user_button.setIcon(QIcon(qta.icon('mdi.account-plus')))
        self.add_user_button.setObjectName("add_user_button")
        self.add_user_button.clicked.connect(self.add_user)
        self.layout.addWidget(self.add_user_button)

        self.setLayout(self.layout)
        self.setStyleSheet("""
            font-size: 16px;
            background-color: #f9f9f9;
            color: #333333;
        """)

    def check_config(self):
        config_path = 'config.toml'
        
        if not os.path.exists(config_path):
            self.show_error('Error', 'config.toml file not found!')
            return
        
        try:
            config = toml.load(config_path)
        except Exception as e:
            self.show_error('Error', f'Error reading config.toml: {e}')
            return
        
        mongo_url = config.get('mongo_url')
        if not mongo_url:
            self.show_error('Error', 'MongoDB URL not found in config.toml!')
        else:
            self.connect_to_mongo(mongo_url)
    
    def connect_to_mongo(self, mongo_url):
        try:
            self.client = MongoClient(mongo_url)
            self.db = self.client['en_chat']
            self.collection = self.db['users']
            self.collection.insert_one({"test": "test"})  # Ensure the collection exists
            self.collection.delete_one({"test": "test"})  # Cleanup the test document
            self.load_users()
        except errors.ConnectionError as e:
            self.show_error('Error', f'Failed to connect to MongoDB: {e}')
        except Exception as e:
            self.show_error('Error', f'An error occurred: {e}')
    
    def load_users(self):
        self.label.setText('Connected to MongoDB and loaded users.')
        users = list(self.collection.find({}, {"_id": 0, "username": 1}))
        
        self.table.setRowCount(len(users))
        self.table.setColumnCount(3)
        self.table.setHorizontalHeaderLabels(['Username', 'Password', 'Actions'])
        
        for row, user in enumerate(users):
            self.table.setItem(row, 0, QTableWidgetItem(user['username']))
            self.table.setItem(row, 1, QTableWidgetItem('******'))
            
            actions_layout = QHBoxLayout()
            delete_button = QPushButton()
            delete_button.setIcon(qta.icon('fa.trash', color='#ffffff'))
            delete_button.setStyleSheet("QPushButton { background-color: #2980b9; border: none; padding: 8px; border-radius: 5px; }")
            delete_button.clicked.connect(lambda _, u=user: self.delete_user(u['username']))
            actions_layout.addWidget(delete_button)
            
            edit_button = QPushButton()
            edit_button.setIcon(qta.icon('fa.edit', color='#ffffff'))
            edit_button.setStyleSheet("QPushButton { background-color: #2980b9; border: none; padding: 8px; border-radius: 5px; }")
            edit_button.clicked.connect(lambda _, u=user: self.edit_user(u['username']))
            actions_layout.addWidget(edit_button)
            
            actions_widget = QWidget()
            actions_widget.setLayout(actions_layout)
            self.table.setCellWidget(row, 2, actions_widget)
    
    def add_user(self):
        dialog = AddEditUserDialog(self)
        if dialog.exec_() == QDialog.Accepted:
            username, password = dialog.get_data()
            hashed_password = hashlib.sha256(password.encode()).hexdigest()
            self.collection.insert_one({'username': username, 'password': hashed_password})
            self.load_users()
    
    def delete_user(self, username):
        self.collection.delete_one({"username": username})
        self.load_users()
    
    def edit_user(self, username):
        user = self.collection.find_one({"username": username})
        if user:
            dialog = AddEditUserDialog(self, username)
            if dialog.exec_() == QDialog.Accepted:
                new_username, new_password = dialog.get_data()
                if new_password:
                    hashed_password = hashlib.sha256(new_password.encode()).hexdigest()
                    self.collection.update_one({'username': username}, {'$set': {'username': new_username, 'password': hashed_password}})
                else:
                    self.collection.update_one({'username': username}, {'$set': {'username': new_username}})
                self.load_users()
    
    def show_error(self, title, message):
        QMessageBox.critical(self, title, message)
        self.label.setText(message)

class AddEditUserDialog(QDialog):
    def __init__(self, parent, username=None):
        super().__init__(parent)
        self.username = username
        self.initUI()
    
    def initUI(self):
        self.setWindowTitle('Add/Edit User')
        self.setGeometry(100, 100, 400, 200)
        
        self.layout = QFormLayout()
        self.username_input = QLineEdit(self)
        self.password_input = QLineEdit(self)
        self.password_input.setEchoMode(QLineEdit.Password)
        
        if self.username:
            self.username_input.setText(self.username)
            self.password_input.setPlaceholderText('Leave empty to keep current password')
        
        self.layout.addRow('Username:', self.username_input)
        self.layout.addRow('Password:', self.password_input)
        
        self.button_layout = QHBoxLayout()
        self.save_button = QPushButton('Save')
        self.save_button.setIcon(qta.icon('fa.save', color='#ffffff'))
        self.save_button.clicked.connect(self.accept)
        self.cancel_button = QPushButton('Cancel')
        self.cancel_button.setIcon(qta.icon('fa.times', color='#ffffff'))
        self.cancel_button.clicked.connect(self.reject)
        
        self.button_layout.addWidget(self.save_button)
        self.button_layout.addWidget(self.cancel_button)
        
        self.layout.addRow(self.button_layout)
        self.setLayout(self.layout)
        self.setStyleSheet("""
            QDialog {
                background-color: #f9f9f9;
                color: #333333;
            }
            QLineEdit {
                background-color: #ffffff;
                border: 1px solid #bdc3c7;
                padding: 5px;
                color: #333333;
            }
            QPushButton {
                background-color: #2980b9;
                color: #ffffff;
                border: none;
                padding: 10px;
                border-radius: 5px;
                font-size: 14px;
            }
            QPushButton:hover {
                background-color: #1c6ca1;
            }
            QFormLayout {
                margin: 20px;
            }
        """)

    def get_data(self):
        return self.username_input.text(), self.password_input.text()

def main():
    app = QApplication(sys.argv)
    
    # Splash screen
    splash = SplashScreen()
    splash.show()
    
    # Simulating initialization time with QTimer
    QTimer.singleShot(3000, splash.close)
    
    checker = ConfigCheckerApp()
    checker.show()
    
    sys.exit(app.exec_())

if __name__ == '__main__':
    main()
