import { defineStore } from "pinia";
import axios from "axios";

export const Logs = defineStore('logs', {
    state: () => ({
        logsData: [],
        filter_logs: [],
        url: 'http://localhost:8000/api'
    }),
    actions: {
        async uploadFile(file) {
            try {
                const formData = new FormData()
                formData.append('file', file)
                
                const response = await axios.post(`${this.url}/log`, formData, {
                    headers: {
                        'Content-Type': 'multipart/form-data'
                    }
                })
                
                return response.data
            } catch (error) {
                console.error('Ошибка загрузки файла:', error)
                throw error
            }
        }
    }
})